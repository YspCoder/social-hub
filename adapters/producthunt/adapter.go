// Package producthunt implements the official Product Hunt API v2 GraphQL read surface.
package producthunt

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "producthunt/graphql-v2"
	platformName     = "producthunt"
	productName      = "producthunt-api"
	apiVersion       = "v2"
	defaultBaseURL   = "https://api.producthunt.com/v2/api"
	defaultUserAgent = "social-hub/producthunt"
	documentationURL = "https://api.producthunt.com/v2/docs"
	graphQLDocsURL   = "https://api-v2-docs.producthunt.com/operation/query/"
	dashboardURL     = "https://www.producthunt.com/v2/oauth/applications"
)

// Settings controls the Product Hunt User-Agent. The API origin is fixed so
// bearer tokens cannot be redirected to a caller-supplied host.
type Settings struct {
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// Adapter implements socialhub.Adapter for Product Hunt API v2.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be producthunt-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{UserAgent: defaultUserAgent}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validOpaque(settings.UserAgent, 256) {
		return invalidArgument("init", "user_agent must be a valid non-empty value")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "account.access_token_ref is required")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, app_id, secret_ref, and token_store are not used by this read adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by Product Hunt GraphQL reads")
		}
		if account.Approval.AccountType != "" || !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval.account_type must be empty and scopes may only contain public, private, or write")
		}
		if len(account.Settings) > 0 {
			var empty struct{}
			if err := socialhub.DecodeSettings(account.Settings, &empty); err != nil {
				return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
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
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil {
		return nil, authenticationError("client", "could not resolve the Product Hunt access token", err, accessToken)
	}
	if !validAccessToken(accessToken) {
		return nil, authenticationError("client", "resolved Product Hunt access token is invalid", nil, accessToken)
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set("User-Agent", settings.UserAgent)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock, accessToken),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, scopes: append([]string(nil), account.Approval.Scopes...),
		api: api, accessToken: accessToken,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
