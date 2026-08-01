// Package x implements the X API v2 adapter.
package x

import (
	"context"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName   = "x/v2"
	defaultAPIURL = "https://api.x.com"
	docURL        = "https://docs.x.com/x-api"
)

// Settings controls X API endpoints. Endpoint overrides are primarily intended
// for local tests and approved compatible gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
}

// Adapter implements socialhub.Adapter for X API v2.
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

// Name returns the registry name.
func (a *Adapter) Name() string { return adapterName }

// Metadata returns the pinned platform product information.
func (a *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name:       adapterName,
		Product:    "api",
		APIVersion: "2",
		DocURL:     docURL,
		VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

// Init validates configuration and shared dependencies.
func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: "init", Cause: err}
	}
	if config.Adapter != "" && config.Adapter != adapterName {
		return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: "init", PlatformMessage: "adapter name mismatch"}
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultAPIURL, TokenURL: defaultAPIURL + "/2/oauth2/token", AuthURL: "https://x.com/i/oauth2/authorize"}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: "init", Cause: err}
		}
	}
	if settings.BaseURL == "" || settings.TokenURL == "" || settings.AuthURL == "" {
		return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: "init", PlatformMessage: "all endpoint URLs are required"}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return &socialhub.Error{Code: socialhub.CodeConflict, Class: socialhub.ClassPermanent, Platform: "x", Op: "init", PlatformMessage: "adapter is closed"}
	}
	a.config = config
	a.options = resolved
	a.settings = settings
	a.ready = true
	return nil
}

// Client creates a client for one configured X account.
func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, &socialhub.Error{Code: socialhub.CodeConflict, Class: socialhub.ClassPermanent, Platform: "x", Op: "client", PlatformMessage: "adapter is not available"}
	}
	account, found := a.config.Account(accountID)
	baseOptions := a.options
	settings := a.settings
	a.mu.RUnlock()
	if !found {
		return nil, &socialhub.Error{Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent, Platform: "x", Op: "client", PlatformMessage: "account is not configured"}
	}

	combined := []socialhub.Option{
		socialhub.WithHTTPClient(baseOptions.HTTPClient),
		socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets),
		socialhub.WithClock(baseOptions.Clock),
	}
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	if account.AccessTokenRef == "" {
		return nil, &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "x", Op: "client", PlatformMessage: "access_token_ref is required"}
	}
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "x", Op: "client", Cause: err}
	}
	tokenSource := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	httpTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokenSource, "x", "api", decodeError)
	if err != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: "client", Cause: err}
	}
	return &Client{accountID: accountID, transport: httpTransport}, nil
}

// Close prevents new clients from being created. Existing clients remain usable.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	a.ready = false
	return nil
}

// OAuth returns an OAuth2 PKCE helper for a configured account.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, &socialhub.Error{Code: socialhub.CodeConflict, Class: socialhub.ClassPermanent, Platform: "x", Op: "oauth", PlatformMessage: "adapter is not available"}
	}
	account, found := a.config.Account(accountID)
	settings := a.settings
	options := a.options
	a.mu.RUnlock()
	if !found {
		return nil, &socialhub.Error{Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent, Platform: "x", Op: "oauth", PlatformMessage: "account is not configured"}
	}
	if account.ClientID == "" {
		return nil, invalidArgument("oauth", "client_id is required")
	}
	client := &OAuthClient{
		ClientID:   account.ClientID,
		AuthURL:    settings.AuthURL,
		TokenURL:   settings.TokenURL,
		HTTPClient: options.HTTPClient,
	}
	if account.SecretRef != "" {
		secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
		if err != nil {
			return nil, &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "x", Product: "oauth2", Op: "oauth", Cause: err}
		}
		client.ClientSecret = secret
	}
	return client, nil
}
