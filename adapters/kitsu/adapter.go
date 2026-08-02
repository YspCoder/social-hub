// Package kitsu implements the Kitsu JSON:API edge API.
package kitsu

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "kitsu/edge"
	productName      = "kitsu-json-api"
	apiVersion       = "edge"
	defaultAPIURL    = "https://kitsu.io/api/edge"
	defaultTokenURL  = "https://kitsu.io/api/oauth/token"
	defaultUserAgent = "social-hub/kitsu"
	documentationURL = "https://hummingbird-me.github.io/api-docs/"
	registrationURL  = "https://kitsu.io/"
)

// Settings controls the Kitsu API and OAuth token endpoints.
type Settings struct {
	APIURL    string `json:"api_url,omitempty" yaml:"api_url,omitempty"`
	TokenURL  string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings contains Kitsu account-specific identifiers.
type AccountSettings struct {
	UserID string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Kitsu's edge JSON:API.
type Adapter struct {
	mu       sync.RWMutex
	config   socialhub.AdapterConfig
	options  socialhub.Options
	settings Settings
	accounts map[socialhub.AccountID]AccountSettings
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
	settings := Settings{APIURL: defaultAPIURL, TokenURL: defaultTokenURL, UserAgent: defaultUserAgent}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.APIURL) || !validEndpoint(settings.TokenURL) || !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "api_url, token_url, or user_agent is invalid")
	}
	accountSettings := make(map[socialhub.AccountID]AccountSettings, len(config.Accounts))
	for _, account := range config.Accounts {
		if account.ClientID != "" && !validCredential(account.ClientID) {
			return invalidArgument("init", "account.client_id is invalid")
		}
		if account.SecretRef != "" && !validReference(account.SecretRef) {
			return invalidArgument("init", "account.secret_ref is invalid")
		}
		if account.AccessTokenRef != "" && !validReference(account.AccessTokenRef) {
			return invalidArgument("init", "account.access_token_ref is invalid")
		}
		if len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "Kitsu OAuth does not expose scopes")
		}
		var specific AccountSettings
		if len(account.Settings) > 0 {
			if err := socialhub.DecodeSettings(account.Settings, &specific); err != nil {
				return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
		}
		if specific.UserID != "" && !validID(specific.UserID) {
			return invalidArgument("init", "account.settings.user_id must be a positive decimal Kitsu user ID")
		}
		accountSettings[account.ID] = specific
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.settings, a.accounts, a.ready = config, resolved, settings, accountSettings, true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	accountSettings := a.accounts[accountID]
	baseOptions, settings := a.options, a.settings
	a.mu.RUnlock()
	if !found {
		return nil, platformError("client", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	combined := []socialhub.Option{
		socialhub.WithHTTPClient(baseOptions.HTTPClient), socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets), socialhub.WithClock(baseOptions.Clock),
	}
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
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
		request.Header.Set("Accept", jsonAPIContentType)
		if accessToken != "" {
			request.Header.Set("Authorization", "Bearer "+accessToken)
		}
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		settings.APIURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "public"}},
		"kitsu", productName, authenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, userID: accountSettings.UserID, clientID: account.ClientID,
		clientSecret: clientSecret, accessToken: accessToken, api: api,
		httpClient: &httpClient, clock: resolved.Clock, tokenURL: settings.TokenURL,
		userAgent: settings.UserAgent,
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
