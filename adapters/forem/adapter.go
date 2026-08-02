// Package forem implements the official Forem API V1.
package forem

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
	adapterName      = "forem/api-v1"
	productName      = "api"
	apiVersion       = "v1.0.0"
	defaultBaseURL   = "https://dev.to"
	documentationURL = "https://developers.forem.com/api/v1"
)

// AccountSettings selects the Forem instance. An empty base URL targets DEV.
type AccountSettings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// Adapter implements socialhub.Adapter for Forem API V1.
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
		if settings.BaseURL == "" {
			settings.BaseURL = defaultBaseURL
		}
		if !validBaseURL(settings.BaseURL) {
			return invalidArgument("init", "account.settings.base_url must be an absolute HTTP(S) URL without credentials, query, or fragment")
		}
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "account.access_token_ref must reference a Forem API key")
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
	if settings.BaseURL == "" {
		settings.BaseURL = defaultBaseURL
	}
	settings.BaseURL = strings.TrimRight(settings.BaseURL, "/")
	apiKey, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if !validHeaderValue(apiKey, 4096) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectCrossOriginRedirect
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: apiKey}},
		"forem", productName, apiKeyAuthenticator{}, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, baseURL: settings.BaseURL, api: api, clock: resolved.Clock}, nil
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

type apiKeyAuthenticator struct{}

func (apiKeyAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if !validHeaderValue(token.AccessToken, 4096) {
		return errors.New("forem: invalid API key")
	}
	request.Header.Set("api-key", token.AccessToken)
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
		return errors.New("forem: too many redirects")
	}
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
var _ transport.Authenticator = apiKeyAuthenticator{}
