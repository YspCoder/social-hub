// Package discourse implements the official Discourse REST API.
package discourse

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "discourse/rest-api"
	productName      = "rest-api"
	apiVersion       = "latest"
	documentationURL = "https://docs.discourse.org/"
)

// AccountSettings identifies one Discourse instance and the API user that the
// configured API key acts as.
type AccountSettings struct {
	BaseURL     string `json:"base_url" yaml:"base_url"`
	APIUsername string `json:"api_username,omitempty" yaml:"api_username,omitempty"`
}

// Adapter implements socialhub.Adapter for Discourse REST API instances.
type Adapter struct {
	mu      sync.RWMutex
	config  socialhub.AdapterConfig
	options socialhub.Options
	ready   bool
	closed  bool
}

func init() {
	socialhub.Register(adapterName, func() socialhub.Adapter { return &Adapter{} })
}

func (adapter *Adapter) Name() string { return adapterName }

func (adapter *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name: adapterName, Product: productName, APIVersion: apiVersion,
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if len(config.Settings) != 0 {
		var settings struct{}
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		var settings AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validBaseURL(settings.BaseURL) {
			return invalidArgument("init", "account.settings.base_url must be an absolute HTTP(S) URL without credentials, query, or fragment")
		}
		hasAPIKey := strings.TrimSpace(account.AccessTokenRef) != ""
		hasWebhookSecret := strings.TrimSpace(account.Webhook.SecretRef) != ""
		if !hasAPIKey && !hasWebhookSecret {
			return invalidArgument("init", "account requires access_token_ref or webhook.secret_ref")
		}
		if hasAPIKey && !validHeaderValue(settings.APIUsername, 512) {
			return invalidArgument("init", "account.settings.api_username is required when access_token_ref is configured")
		}
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	adapter.config, adapter.options, adapter.ready = config, resolved, true
	return nil
}

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	baseOptions := adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("client", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}

	combined := []socialhub.Option{
		socialhub.WithHTTPClient(baseOptions.HTTPClient), socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets), socialhub.WithClock(baseOptions.Clock),
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	settings.APIUsername = strings.TrimSpace(settings.APIUsername)

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectCrossOriginRedirect
	var api *transport.Client
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		apiKey, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		apiKey = strings.TrimSpace(apiKey)
		if !validHeaderValue(apiKey, 4096) {
			return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
		}
		tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: apiKey}}
		api, err = transport.NewWithAuthenticator(
			strings.TrimRight(settings.BaseURL, "/"), &httpClient, tokens, "discourse", productName,
			apiKeyAuthenticator{username: settings.APIUsername}, decodeHTTPError,
		)
		if err != nil {
			return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	webhookSecret := ""
	if strings.TrimSpace(account.Webhook.SecretRef) != "" {
		webhookSecret, err = resolveSecret(ctx, resolved.Secrets, account.Webhook.SecretRef, "client")
		if err != nil {
			return nil, err
		}
	}
	return &Client{
		accountID: accountID, baseURL: strings.TrimRight(settings.BaseURL, "/"), apiUsername: settings.APIUsername,
		api: api, webhookSecret: webhookSecret, clock: resolved.Clock,
		uploads: make(map[string]*uploadState), media: make(map[string]socialhub.Media),
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

type apiKeyAuthenticator struct{ username string }

func (authenticator apiKeyAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if !validHeaderValue(token.AccessToken, 4096) || !validHeaderValue(authenticator.username, 512) {
		return errors.New("discourse: invalid API credentials")
	}
	request.Header.Set("Api-Key", token.AccessToken)
	request.Header.Set("Api-Username", authenticator.username)
	return nil
}

func validBaseURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validHeaderValue(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func rejectCrossOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	origin := via[0].URL
	if !strings.EqualFold(request.URL.Scheme, origin.Scheme) || !strings.EqualFold(request.URL.Host, origin.Host) {
		return http.ErrUseLastResponse
	}
	if len(via) >= 10 {
		return errors.New("discourse: too many redirects")
	}
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
var _ transport.Authenticator = apiKeyAuthenticator{}
