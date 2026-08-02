// Package anilist implements the official AniList GraphQL API v2.
package anilist

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName       = "anilist/graphql-v2"
	productName       = "anilist-api"
	apiVersion        = "v2"
	defaultGraphQLURL = "https://graphql.anilist.co"
	defaultAuthURL    = "https://anilist.co/api/v2/oauth/authorize"
	defaultTokenURL   = "https://anilist.co/api/v2/oauth/token"
	defaultUserAgent  = "social-hub/anilist"
	documentationURL  = "https://docs.anilist.co/"
	authorizationURL  = "https://docs.anilist.co/guide/auth/"
	registrationURL   = "https://anilist.co/settings/developer"
)

// Settings controls the AniList GraphQL and OAuth endpoints.
type Settings struct {
	GraphQLURL string `json:"graphql_url,omitempty" yaml:"graphql_url,omitempty"`
	AuthURL    string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL   string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	UserAgent  string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// Adapter implements socialhub.Adapter for AniList API v2.
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
		GraphQLURL: defaultGraphQLURL, AuthURL: defaultAuthURL,
		TokenURL: defaultTokenURL, UserAgent: defaultUserAgent,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.GraphQLURL) || !validEndpoint(settings.AuthURL) ||
		!validEndpoint(settings.TokenURL) || !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "graphql_url, auth_url, token_url, or user_agent is invalid")
	}
	for _, account := range config.Accounts {
		if account.ClientID != "" && !validCredential(account.ClientID) {
			return invalidArgument("init", "account.client_id is invalid")
		}
		if account.SecretRef != "" && (!validReference(account.SecretRef) || account.ClientID == "") {
			return invalidArgument("init", "account.secret_ref requires a valid client_id")
		}
		if account.AccessTokenRef != "" && !validReference(account.AccessTokenRef) {
			return invalidArgument("init", "account.access_token_ref is invalid")
		}
		if len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "AniList OAuth does not support scopes")
		}
		if len(account.Settings) > 0 {
			var empty struct{}
			if err := socialhub.DecodeSettings(account.Settings, &empty); err != nil {
				return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
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
	clientSecret, err := resolveOptionalSecret(ctx, resolved.Secrets, account.SecretRef)
	if err != nil || (clientSecret != "" && !validCredential(clientSecret)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	accessToken, err := resolveOptionalSecret(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil || (accessToken != "" && !validCredential(accessToken)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, _ socialhub.Token) error {
		request.Header.Set("User-Agent", settings.UserAgent)
		if accessToken != "" {
			request.Header.Set("Authorization", "Bearer "+accessToken)
		}
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		settings.GraphQLURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "public"}},
		"anilist", productName, authenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, clientID: account.ClientID, clientSecret: clientSecret,
		accessToken: accessToken, api: api, httpClient: &httpClient, clock: resolved.Clock,
		authURL: settings.AuthURL, tokenURL: settings.TokenURL, userAgent: settings.UserAgent,
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
