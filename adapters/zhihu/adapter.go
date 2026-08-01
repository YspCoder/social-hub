// Package zhihu implements the documented Zhihu Data Open Platform APIs.
package zhihu

import (
	"context"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "zhihu/data-api"
	defaultAPIURL    = "https://developer.zhihu.com"
	defaultAuthURL   = "https://openapi.zhihu.com/authorize"
	defaultTokenURL  = "https://openapi.zhihu.com/access_token"
	docURL           = "https://developer.zhihu.com/docs"
	approvalURL      = "https://developer.zhihu.com/profile"
	maxResponseBytes = 1 << 20
)

// CapabilitySearch identifies Zhihu's documented site-search API.
const CapabilitySearch socialhub.Capability = "content_search"

// CapabilityHotList identifies Zhihu's documented hot-list API.
const CapabilityHotList socialhub.Capability = "hot_list"

// Settings controls Zhihu Data Open Platform and OAuth endpoints.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

type accountSettings struct {
	Approved      bool   `json:"approved,omitempty" yaml:"approved,omitempty"`
	OAuthTokenRef string `json:"oauth_token_ref,omitempty" yaml:"oauth_token_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Zhihu Data Open Platform.
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

func (a *Adapter) Name() string { return adapterName }

func (a *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name:       adapterName,
		Product:    "data-api",
		APIVersion: "v1",
		DocURL:     docURL,
		VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultAPIURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.BaseURL == "" || settings.AuthURL == "" || settings.TokenURL == "" {
		return invalidArgument("init", "base_url, auth_url, and token_url are required")
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef == "" {
			return invalidArgument("init", "access_token_ref (Access Secret) is required for every account")
		}
		if (account.ClientID == "") != (account.SecretRef == "") {
			return invalidArgument("init", "client_id (app_id) and secret_ref (app_key) must be configured together")
		}
		var typed accountSettings
		if len(account.Settings) > 0 {
			if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
				return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return wrapError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config = config
	a.options = resolved
	a.settings = settings
	a.ready = true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, wrapError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions := a.options
	settings := a.settings
	a.mu.RUnlock()
	if !found {
		return nil, wrapError("client", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	combined := []socialhub.Option{
		socialhub.WithHTTPClient(baseOptions.HTTPClient),
		socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets),
		socialhub.WithClock(baseOptions.Clock),
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	accessSecret, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed accountSettings
	if len(account.Settings) > 0 {
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	var oauthToken string
	if typed.OAuthTokenRef != "" {
		oauthToken, err = resolved.Secrets.Resolve(ctx, typed.OAuthTokenRef)
		if err != nil {
			return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
	}
	httpTransport, err := transport.NewWithAuthenticator(
		settings.BaseURL,
		resolved.HTTPClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessSecret, TokenType: "Bearer"}},
		"zhihu",
		"data-api",
		requestAuthenticator{clock: resolved.Clock},
		decodeHTTPError,
	)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{accountID: accountID, transport: httpTransport, clock: resolved.Clock, oauthToken: oauthToken, approved: typed.Approved}
	client.search = &SearchService{client: client}
	return client, nil
}

// OAuth returns the documented OAuth 2.0 authorization-code helper.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, wrapError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	settings := a.settings
	options := a.options
	a.mu.RUnlock()
	if !found {
		return nil, wrapError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if account.ClientID == "" || account.SecretRef == "" {
		return nil, approvalError("oauth", "OAuth app_id and app_key require separate application approval")
	}
	appKey, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil {
		return nil, wrapError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return &OAuthClient{AppID: account.ClientID, AppKey: appKey, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL, HTTPClient: options.HTTPClient, Clock: options.Clock}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = false
	a.closed = true
	return nil
}
