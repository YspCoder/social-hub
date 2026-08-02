// Package deviantart implements DeviantArt's official OAuth API v1.
package deviantart

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "deviantart/api-v1-20240701"
	productName      = "deviantart-api"
	apiVersion       = "1.20240701"
	defaultBaseURL   = "https://www.deviantart.com/api/v1/oauth2"
	defaultAuthURL   = "https://www.deviantart.com/oauth2/authorize"
	defaultTokenURL  = "https://www.deviantart.com/oauth2/token"
	defaultRevokeURL = "https://www.deviantart.com/oauth2/revoke"
	documentationURL = "https://deviantart.readme.io/"
	defaultUserAgent = "social-hub/deviantart-api-v1-20240701"
	minorVersion     = "20240701"
)

// Settings controls DeviantArt API and OAuth endpoints.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL   string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL  string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	RevokeURL string `json:"revoke_url,omitempty" yaml:"revoke_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings identifies the DeviantArt account used when a common
// workflow omits an explicit username.
type AccountSettings struct {
	Username string `json:"username" yaml:"username"`
	UserID   string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
}

// Adapter implements socialhub.Adapter for DeviantArt API v1.
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
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{
		BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL,
		RevokeURL: defaultRevokeURL, UserAgent: defaultUserAgent,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL, settings.RevokeURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all DeviantArt endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	if !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "user_agent is invalid")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validUsername(typed.Username) {
			return invalidArgument("init", "account.settings.username is required and must be a valid path segment")
		}
		if typed.UserID != "" && !validResourceID(typed.UserID) {
			return invalidArgument("init", "account.settings.user_id must be a valid DeviantArt user ID when set")
		}
		if !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval scopes must be unique documented DeviantArt OAuth scopes")
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
	if strings.TrimSpace(account.AccessTokenRef) == "" {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, socialhub.ErrUnauthenticated)
	}
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: strings.TrimSpace(accessToken), TokenType: "Bearer"}}
	api, err := transport.New(settings.BaseURL, &httpClient, tokens, "deviantart", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, username: typed.Username, userID: typed.UserID, scopes: append([]string(nil), account.Approval.Scopes...),
		api: api, clock: resolved.Clock, userAgent: settings.UserAgent,
	}, nil
}

// OAuth returns an OAuth 2.1 PKCE helper. A missing secret_ref selects a
// public client; confidential clients resolve and send their client secret.
func (adapter *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	settings, options := adapter.settings, adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(account.ClientID, 512) {
		return nil, invalidArgument("oauth", "client_id is required")
	}
	secret := ""
	var err error
	if strings.TrimSpace(account.SecretRef) != "" {
		secret, err = resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
		if err != nil {
			return nil, err
		}
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: settings.TokenURL, RevokeURL: settings.RevokeURL, UserAgent: settings.UserAgent,
		HTTPClient: &httpClient, Clock: options.Clock,
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

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
