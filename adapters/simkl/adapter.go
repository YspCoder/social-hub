// Package simkl implements the official Simkl API v1.
package simkl

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName        = "simkl/v1"
	productName        = "simkl-api"
	apiVersion         = "1.0.0"
	defaultAPIURL      = "https://api.simkl.com"
	defaultCDNURL      = "https://data.simkl.in"
	defaultAuthURL     = "https://simkl.com/oauth/authorize"
	defaultTokenURL    = "https://api.simkl.com/oauth/token"
	defaultAppName     = "social-hub"
	defaultAppVersion  = "0.1"
	defaultUserAgent   = "social-hub/simkl"
	documentationURL   = "https://api.simkl.org/"
	developerPortalURL = "https://simkl.com/settings/developer/"
)

// Settings controls Simkl's API, CDN, OAuth, and application-identification values.
type Settings struct {
	APIURL     string `json:"api_url,omitempty" yaml:"api_url,omitempty"`
	CDNURL     string `json:"cdn_url,omitempty" yaml:"cdn_url,omitempty"`
	AuthURL    string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL   string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	AppName    string `json:"app_name,omitempty" yaml:"app_name,omitempty"`
	AppVersion string `json:"app_version,omitempty" yaml:"app_version,omitempty"`
	UserAgent  string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// Adapter implements socialhub.Adapter for Simkl API v1.
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
		APIURL: defaultAPIURL, CDNURL: defaultCDNURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL,
		AppName: defaultAppName, AppVersion: defaultAppVersion, UserAgent: defaultUserAgent,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.APIURL) || !validEndpoint(settings.CDNURL) ||
		!validEndpoint(settings.AuthURL) || !validEndpoint(settings.TokenURL) ||
		!validApplicationValue(settings.AppName) || !validApplicationValue(settings.AppVersion) ||
		!validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "API/CDN/OAuth URLs or application identification is invalid")
	}
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
		if account.ClientID == "" && (account.SecretRef != "" || account.AccessTokenRef != "") {
			return invalidArgument("init", "account.client_id is required when account credentials are configured")
		}
		if len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "Simkl OAuth does not expose configurable scopes")
		}
		if len(account.Settings) > 0 {
			return invalidArgument("init", "Simkl accounts do not define adapter-specific settings")
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
	publicAuthenticator := transport.AuthenticatorFunc(func(request *http.Request, _ socialhub.Token) error {
		query := request.URL.Query()
		if account.ClientID != "" {
			query.Set("client_id", account.ClientID)
		}
		query.Set("app-name", settings.AppName)
		query.Set("app-version", settings.AppVersion)
		request.URL.RawQuery = query.Encode()
		request.Header.Set("User-Agent", settings.UserAgent)
		return nil
	})
	userAuthenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if err := publicAuthenticator.Authenticate(request, token); err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+accessToken)
		return nil
	})
	cdnAuthenticator := transport.AuthenticatorFunc(func(request *http.Request, _ socialhub.Token) error {
		query := request.URL.Query()
		if account.ClientID != "" {
			query.Set("client_id", account.ClientID)
		}
		query.Set("app-name", settings.AppName)
		query.Set("app-version", settings.AppVersion)
		request.URL.RawQuery = query.Encode()
		request.Header.Set("User-Agent", settings.UserAgent)
		return nil
	})
	tokenSource := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "public"}}
	api, err := transport.NewWithAuthenticator(settings.APIURL, &httpClient, tokenSource, "simkl", productName, publicAuthenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	userAPI, err := transport.NewWithAuthenticator(settings.APIURL, &httpClient, tokenSource, "simkl", productName, userAuthenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	cdn, err := transport.NewWithAuthenticator(settings.CDNURL, &httpClient, tokenSource, "simkl", productName, cdnAuthenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, clientID: account.ClientID, clientSecret: clientSecret, accessToken: accessToken,
		api: api, userAPI: userAPI, cdn: cdn, httpClient: &httpClient, clock: resolved.Clock,
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
