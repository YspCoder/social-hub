// Package tmdb implements the official TMDB API v3.
package tmdb

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "tmdb/v3"
	productName      = "tmdb-api"
	apiVersion       = "v3"
	defaultBaseURL   = "https://api.themoviedb.org/3"
	defaultAuthURL   = "https://www.themoviedb.org/authenticate"
	defaultUserAgent = "social-hub/tmdb"
	documentationURL = "https://developer.themoviedb.org/reference/getting-started"
)

// Settings controls TMDB API, browser approval, and User-Agent values.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL   string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings contains user and guest session defaults.
type AccountSettings struct {
	AccountID       int64  `json:"account_id,omitempty" yaml:"account_id,omitempty"`
	GuestSessionRef string `json:"guest_session_ref,omitempty" yaml:"guest_session_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for TMDB API v3.
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
		if account.SecretRef == "" && !validCredential(account.ClientID) {
			return invalidArgument("init", "secret_ref for a bearer token or client_id for a v3 API key is required")
		}
		if account.SecretRef != "" && !validReference(account.SecretRef) {
			return invalidArgument("init", "secret_ref is invalid")
		}
		if account.AccessTokenRef != "" && !validReference(account.AccessTokenRef) {
			return invalidArgument("init", "access_token_ref is invalid")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.AccountID < 0 || (typed.GuestSessionRef != "" && !validReference(typed.GuestSessionRef)) {
			return invalidArgument("init", "account_id or guest_session_ref is invalid")
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
	bearerToken, err := resolveOptionalSecret(ctx, resolved.Secrets, account.SecretRef)
	if err != nil || (bearerToken != "" && !validCredential(bearerToken)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	apiKey := account.ClientID
	if bearerToken == "" && !validCredential(apiKey) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	sessionID, err := resolveOptionalSecret(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil || (sessionID != "" && !validCredential(sessionID)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	guestSessionID, err := resolveOptionalSecret(ctx, resolved.Secrets, typed.GuestSessionRef)
	if err != nil || (guestSessionID != "" && !validCredential(guestSessionID)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	credential := firstNonEmpty(bearerToken, apiKey)
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("User-Agent", settings.UserAgent)
		if bearerToken != "" {
			request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		} else {
			query := request.URL.Query()
			query.Set("api_key", token.AccessToken)
			request.URL.RawQuery = query.Encode()
		}
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, resolved.HTTPClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: credential, TokenType: "Bearer"}},
		"tmdb", productName, authenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, tmdbAccountID: typed.AccountID, sessionID: sessionID, guestSessionID: guestSessionID,
		api: api, authURL: settings.AuthURL,
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
