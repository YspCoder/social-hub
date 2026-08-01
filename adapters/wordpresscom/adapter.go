// Package wordpresscom implements the official WordPress.com REST API.
package wordpresscom

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "wordpress.com/rest-v1.1"
	productName      = "rest-api"
	apiVersion       = "v1.1"
	defaultBaseURL   = "https://public-api.wordpress.com/rest/v1.1"
	defaultAuthURL   = "https://public-api.wordpress.com/oauth2/authorize"
	defaultTokenURL  = "https://public-api.wordpress.com/oauth2/token"
	documentationURL = "https://developer.wordpress.com/docs/api/"
)

// Settings controls WordPress.com REST and OAuth endpoints.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings selects a WordPress.com or Jetpack-connected site.
type AccountSettings struct {
	Site   string `json:"site" yaml:"site"`
	UserID string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
}

// Adapter implements socialhub.Adapter for WordPress.com REST API v1.1.
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
	settings := Settings{BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all WordPress.com endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil || !validSite(typed.Site) || typed.UserID != "" && !validID(typed.UserID) {
			return invalidArgument("init", "account.settings.site must be a site ID or hostname and user_id must be a positive integer when set")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	publicTokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "anonymous"}}
	publicAPI, err := transport.NewWithAuthenticator(
		settings.BaseURL, &httpClient, publicTokens, "wordpress.com", productName,
		transport.AuthenticatorFunc(func(*http.Request, socialhub.Token) error { return nil }), decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	var userAPI *transport.Client
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
		userAPI, err = transport.New(settings.BaseURL, &httpClient, tokens, "wordpress.com", productName, decodeHTTPError)
		if err != nil {
			return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	return &Client{
		accountID: accountID, site: strings.TrimSpace(typed.Site), userID: typed.UserID,
		public: publicAPI, user: userAPI, scopes: append([]string(nil), account.Approval.Scopes...),
		clock: resolved.Clock, uploads: make(map[string]*uploadState),
	}, nil
}

// OAuth returns a WordPress.com OAuth2 authorization-code helper.
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
	if !validOpaque(account.ClientID, 512) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, Site: typed.Site,
		AuthURL: settings.AuthURL, TokenURL: settings.TokenURL, HTTPClient: &httpClient,
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

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validSite(value string) bool {
	value = strings.TrimSpace(value)
	if validID(value) {
		return true
	}
	if value == "" || len(value) > 253 || strings.ContainsAny(value, "/\\?#:@") {
		return false
	}
	parsed, err := url.Parse("https://" + value)
	return err == nil && parsed.Hostname() == value && strings.Contains(value, ".")
}

func validOpaque(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
