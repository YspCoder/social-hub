// Package unitystatistics implements Unity Advertising Statistics API v2.
package unitystatistics

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "unity/advertising-statistics-api-v2"
	platformName     = "unity"
	productName      = "advertising-statistics-api"
	apiVersion       = "v2"
	contractVersion  = "v2.0 latest"
	defaultBaseURL   = "https://services.api.unity.com"
	documentationURL = "https://services.docs.unity.com/statistics/v2/"
)

// Settings controls the Statistics API endpoint. Overrides are intended for
// tests and controlled HTTP gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings binds one social-hub account to one Unity organization.
type AccountSettings struct {
	OrganizationID string `json:"organization_id" yaml:"organization_id"`
}

// Adapter implements socialhub.Adapter for Unity Advertising Statistics API v2.
type Adapter struct {
	mu       sync.RWMutex
	config   socialhub.AdapterConfig
	options  socialhub.Options
	settings Settings
	ready    bool
	closed   bool
}

func init() {
	socialhub.Register(adapterName, func() socialhub.Adapter { return &Adapter{} })
}

func (adapter *Adapter) Name() string { return adapterName }

func (adapter *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name: adapterName, Product: productName, APIVersion: apiVersion,
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if config.Product != "" && config.Product != productName {
		return invalidArgument("init", "product must be advertising-statistics-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || strings.HasSuffix(settings.BaseURL, "/") {
		return invalidArgument("init", "settings.base_url must be an absolute HTTP(S) URL without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validOrganizationID(typed.OrganizationID) {
			return invalidArgument("init", "account.settings.organization_id must be a positive numeric Organization Core ID")
		}
		bearer := strings.TrimSpace(account.AccessTokenRef) != ""
		basic := validBasicKeyID(account.ClientID) && strings.TrimSpace(account.SecretRef) != ""
		if bearer == basic {
			return invalidArgument("init", "configure exactly one of access_token_ref or client_id with secret_ref")
		}
		if bearer && (account.ClientID != "" || account.SecretRef != "") {
			return invalidArgument("init", "client_id and secret_ref cannot be combined with access_token_ref")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Unity Advertising Statistics API v2")
		}
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	adapter.config, adapter.options, adapter.settings, adapter.ready = config, resolved, settings, true
	return nil
}

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	baseOptions, settings := adapter.options, adapter.settings
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("client", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	combined := []socialhub.Option{
		socialhub.WithHTTPClient(baseOptions.HTTPClient), socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets), socialhub.WithClock(baseOptions.Clock),
	}
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}

	var tokens socialhub.TokenSource
	var authenticator transport.Authenticator
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		bearer, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: bearer, TokenType: "Bearer"}}
		authenticator = transport.BearerAuthenticator{}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		keyID := account.ClientID
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: secret, TokenType: "Basic"}}
		authenticator = transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
			if !validBasicKeyID(keyID) || !validOpaque(token.AccessToken, 16384) {
				return socialhub.ErrUnauthenticated
			}
			request.SetBasicAuth(keyID, token.AccessToken)
			return nil
		})
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	api, err := transport.NewWithAuthenticator(settings.BaseURL, httpClient, tokens, platformName, productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, organizationID: typed.OrganizationID, api: api, httpClient: httpClient}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16384) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

var _ socialhub.Adapter = (*Adapter)(nil)
