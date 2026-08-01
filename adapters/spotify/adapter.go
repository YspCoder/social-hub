// Package spotify implements the Spotify Web API v1.
package spotify

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName     = "spotify/web-api-v1"
	productName     = "spotify-web-api"
	apiVersion      = "v1"
	defaultBaseURL  = "https://api.spotify.com/v1"
	defaultAuthURL  = "https://accounts.spotify.com/authorize"
	defaultTokenURL = "https://accounts.spotify.com/api/token"
	docURL          = "https://developer.spotify.com/documentation/web-api/reference"
)

// Settings controls Spotify Web API and OAuth endpoints.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings optionally records Spotify's immutable account-linking ID.
type AccountSettings struct {
	AccountID string `json:"account_id,omitempty" yaml:"account_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Spotify Web API v1.
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
		Name: adapterName, Product: productName, APIVersion: apiVersion,
		DocURL: docURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformErrorWithCause("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformErrorWithCause("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all Spotify endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every account")
		}
		if len(account.Approval.Scopes) > 0 && !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "account approval contains an invalid or duplicate Spotify scope")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformErrorWithCause("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.AccountID != "" && !validSpotifyID(typed.AccountID) {
			return invalidArgument("init", "account.settings.account_id is invalid")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformErrorWithCause("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.settings, a.ready = config, resolved, settings, true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformErrorWithCause("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions, settings := a.options, a.settings
	a.mu.RUnlock()
	if !found {
		return nil, platformErrorWithCause("client", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
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
	if err != nil || strings.TrimSpace(accessToken) == "" {
		return nil, platformErrorWithCause("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformErrorWithCause("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	api, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "spotify", productName, decodeHTTPError)
	if err != nil {
		return nil, platformErrorWithCause("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	return &Client{
		accountID: accountID, spotifyAccountID: typed.AccountID, accountType: account.Approval.AccountType,
		scopes: append([]string(nil), account.Approval.Scopes...), api: api, apiBaseURL: baseURL, clock: resolved.Clock,
	}, nil
}

// OAuth returns a Spotify OAuth 2.0 helper for the configured application.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformErrorWithCause("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	settings, options := a.settings, a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformErrorWithCause("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if strings.TrimSpace(account.ClientID) == "" {
		return nil, invalidArgument("oauth", "client_id is required")
	}
	var secret string
	if strings.TrimSpace(account.SecretRef) != "" {
		var err error
		secret, err = options.Secrets.Resolve(ctx, account.SecretRef)
		if err != nil || strings.TrimSpace(secret) == "" {
			return nil, platformErrorWithCause("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: settings.TokenURL, HTTPClient: options.HTTPClient, Clock: options.Clock,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

var _ socialhub.Adapter = (*Adapter)(nil)
