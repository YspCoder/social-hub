// Package lastfm implements Last.fm Web Services API 2.0.
package lastfm

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "lastfm/web-services-v2"
	productName      = "lastfm-web-services"
	apiVersion       = "2.0"
	defaultBaseURL   = "https://ws.audioscrobbler.com/2.0/"
	defaultAuthURL   = "https://www.last.fm/api/auth/"
	documentationURL = "https://www.last.fm/api"
)

// Settings controls the Last.fm API and browser-authorization origins.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
}

// AccountSettings contains an optional default Last.fm username.
type AccountSettings struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
}

// Adapter implements socialhub.Adapter for Last.fm Web Services API 2.0.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
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
	settings := Settings{BaseURL: defaultBaseURL, AuthURL: defaultAuthURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.AuthURL) {
		return invalidArgument("init", "base_url and auth_url must be absolute HTTP(S) URLs without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		if !validAPIKey(account.ClientID) {
			return invalidArgument("init", "account.client_id must be a 32-character Last.fm API key")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.Username != "" && !validText(typed.Username, 255) {
			return invalidArgument("init", "account.settings.username is invalid")
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.settings, a.ready = config, resolved, settings, true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions, settings := a.options, a.settings
	a.mu.RUnlock()
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	apiSecret, err := resolveOptionalSecret(ctx, resolved.Secrets, account.SecretRef)
	if err != nil || (apiSecret != "" && !validCredential(apiSecret)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	sessionKey, err := resolveOptionalSecret(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil || (sessionKey != "" && !validCredential(sessionKey)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, resolved.HTTPClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: account.ClientID}},
		"lastfm", productName, transport.AuthenticatorFunc(func(*http.Request, socialhub.Token) error { return nil }),
		decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	authURL, _ := url.Parse(settings.AuthURL)
	return &Client{
		accountID: accountID, apiKey: account.ClientID, apiSecret: apiSecret,
		sessionKey: sessionKey, username: typed.Username, authURL: authURL, api: api,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func resolveOptionalSecret(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	if reference == "" {
		return "", nil
	}
	return resolver.Resolve(ctx, reference)
}

var _ socialhub.Adapter = (*Adapter)(nil)
