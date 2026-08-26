// Package tradedoubler implements the Tradedoubler Products API 1.0 publisher
// read surface.
package tradedoubler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "tradedoubler/products-api-v1.0"
	platformName     = "tradedoubler"
	productName      = "products-api"
	apiVersion       = "1.0"
	defaultBaseURL   = "https://api.tradedoubler.com"
	documentationURL = "https://dev.tradedoubler.com/products/publisher/"
)

// Settings controls the Products API endpoint. BaseURL overrides are intended
// for controlled contract-verification gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// Adapter implements socialhub.Adapter for the Tradedoubler Products API.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be products-api")
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
	if !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an absolute HTTP(S) URL without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) || account.ClientID != "" || account.SecretRef != "" || account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "access_token_ref is required; client_id, secret_ref, app_id, and token_store are not used")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by Products API read workflows")
		}
		if len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth scopes are not used by Products API token authentication")
		}
		if account.Approval.AccountType != "" && !validOpaque(account.Approval.AccountType, 256) {
			return invalidArgument("init", "approval.account_type is invalid")
		}
		if len(account.Settings) > 0 {
			return invalidArgument("init", "account-specific settings are not used by Products API")
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
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	token, err := resolvePublisherToken(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	configuredSecrets := []string{token}
	errorSecrets := func() []string { return append([]string(nil), configuredSecrets...) }
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: token, TokenType: "Products"}}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, httpClient, tokens, platformName, productName,
		transport.QueryAuthenticator("token"), newHTTPErrorDecoder(resolved.Clock, errorSecrets),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api, errorSecrets: errorSecrets, approval: account.Approval}, nil
}

func resolvePublisherToken(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validPublisherToken(value) {
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

var _ socialhub.Adapter = (*Adapter)(nil)
