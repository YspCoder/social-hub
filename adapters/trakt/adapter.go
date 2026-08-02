// Package trakt implements the official Trakt API v2.
package trakt

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "trakt/v2"
	productName      = "trakt-api"
	apiVersion       = "2"
	defaultBaseURL   = "https://api.trakt.tv"
	defaultAuthURL   = "https://auth.trakt.tv"
	defaultUserAgent = "social-hub/trakt"
	documentationURL = "https://docs.trakt.tv"
)

// Settings controls Trakt API and OAuth origins and the required User-Agent.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL   string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings contains an optional default Trakt username or slug.
type AccountSettings struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
}

// Adapter implements socialhub.Adapter for Trakt API v2.
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
	settings := Settings{BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, UserAgent: defaultUserAgent}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.AuthURL) || !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "base_url, auth_url, or user_agent is invalid")
	}
	for _, account := range config.Accounts {
		if !validCredential(account.ClientID) {
			return invalidArgument("init", "account.client_id is required")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.Username != "" && !validIdentifier(typed.Username, 255) {
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
	clientSecret, err := resolveOptionalSecret(ctx, resolved.Secrets, account.SecretRef)
	if err != nil || (clientSecret != "" && !validCredential(clientSecret)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	accessToken, err := resolveOptionalSecret(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil || (accessToken != "" && !validCredential(accessToken)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	authenticated := accessToken != ""
	transportToken := accessToken
	if transportToken == "" {
		transportToken = account.ClientID
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("trakt-api-key", account.ClientID)
		request.Header.Set("trakt-api-version", apiVersion)
		request.Header.Set("User-Agent", settings.UserAgent)
		if authenticated {
			request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		}
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, resolved.HTTPClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: transportToken, TokenType: "Bearer"}},
		"trakt", productName, authenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, clientID: account.ClientID, clientSecret: clientSecret,
		username: typed.Username, authenticated: authenticated, api: api,
		httpClient: resolved.HTTPClient, clock: resolved.Clock, authURL: settings.AuthURL, userAgent: settings.UserAgent,
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
