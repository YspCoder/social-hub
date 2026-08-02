// Package letterboxd implements the official Letterboxd API v0.
package letterboxd

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "letterboxd/api-v0"
	productName      = "letterboxd-api"
	apiVersion       = "v0"
	defaultBaseURL   = "https://api.letterboxd.com/api/v0"
	defaultAuthURL   = "https://letterboxd.com/api/v0/auth/authorize"
	defaultTokenURL  = "https://api.letterboxd.com/api/v0/auth/token"
	defaultRevokeURL = "https://api.letterboxd.com/api/v0/auth/revoke"
	defaultUserAgent = "social-hub/letterboxd"
	documentationURL = "https://api-docs.letterboxd.com/"
	approvalURL      = "https://letterboxd.com/api-beta/"
)

// TokenKind identifies whether the configured token was issued by a client
// credentials flow or an authorization code flow.
type TokenKind string

const (
	TokenClient TokenKind = "client"
	TokenUser   TokenKind = "user"
)

// Settings controls Letterboxd API and OAuth endpoints.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL   string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL  string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	RevokeURL string `json:"revoke_url,omitempty" yaml:"revoke_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings describes the actor represented by access_token_ref.
type AccountSettings struct {
	TokenKind TokenKind `json:"token_kind,omitempty" yaml:"token_kind,omitempty"`
}

// Adapter implements socialhub.Adapter for Letterboxd API v0.
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
	settings := Settings{
		BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL,
		RevokeURL: defaultRevokeURL, UserAgent: defaultUserAgent,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.AuthURL) ||
		!validEndpoint(settings.TokenURL) || !validEndpoint(settings.RevokeURL) || !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "base_url, auth_url, token_url, revoke_url, or user_agent is invalid")
	}
	for _, account := range config.Accounts {
		if !validCredential(account.ClientID) || !validReference(account.SecretRef) {
			return invalidArgument("init", "account.client_id and account.secret_ref are required")
		}
		if account.AccessTokenRef != "" && !validReference(account.AccessTokenRef) {
			return invalidArgument("init", "account.access_token_ref is invalid")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.TokenKind == "" {
			typed.TokenKind = TokenClient
		}
		if typed.TokenKind != TokenClient && typed.TokenKind != TokenUser {
			return invalidArgument("init", "account.settings.token_kind must be client or user")
		}
		if account.AccessTokenRef == "" && typed.TokenKind == TokenUser {
			return invalidArgument("init", "a user token requires account.access_token_ref")
		}
		if !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "account approval contains an invalid, duplicate, or first-party scope")
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
	clientSecret, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || !validCredential(clientSecret) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	accessToken, err := resolveOptionalSecret(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil || (accessToken != "" && !validCredential(accessToken)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if typed.TokenKind == "" {
		typed.TokenKind = TokenClient
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	transportToken := accessToken
	if transportToken == "" {
		transportToken = account.ClientID
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", settings.UserAgent)
		if accessToken != "" {
			request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		}
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: transportToken, TokenType: "Bearer"}},
		"letterboxd", productName, authenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, clientID: account.ClientID, clientSecret: clientSecret,
		tokenKind: typed.TokenKind, accessToken: accessToken,
		scopes: append([]string(nil), account.Approval.Scopes...), api: api,
		httpClient: &httpClient, clock: resolved.Clock, userAgent: settings.UserAgent,
		authURL: settings.AuthURL, tokenURL: settings.TokenURL, revokeURL: settings.RevokeURL,
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

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
