// Package stackexchange implements Stack Exchange API v2.3.
package stackexchange

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
	adapterName      = "stackexchange/api-v2.3"
	productName      = "stack-exchange-api"
	apiVersion       = "v2.3"
	defaultBaseURL   = "https://api.stackexchange.com/2.3"
	defaultAuthURL   = "https://stackoverflow.com/oauth"
	defaultTokenURL  = "https://stackoverflow.com/oauth/access_token/json"
	defaultUserAgent = "social-hub/stackexchange"
	documentationURL = "https://api.stackexchange.com/docs"
)

// Settings controls Stack Exchange API and OAuth endpoints.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL   string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL  string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings selects one Stack Exchange site and, optionally, its
// authorized user for common user-scoped operations.
type AccountSettings struct {
	Site      string `json:"site" yaml:"site"`
	UserID    string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
	APIKeyRef string `json:"api_key_ref,omitempty" yaml:"api_key_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Stack Exchange API v2.3.
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
	settings := Settings{BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL, UserAgent: defaultUserAgent}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all Stack Exchange endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	if !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "settings.user_agent must be a non-empty single-line value up to 256 characters")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if (account.AppID == "" && !validOpaque(typed.APIKeyRef, 1024)) || (account.AppID != "" && !validOpaque(account.AppID, 512)) {
			return invalidArgument("init", "app_id or account.settings.api_key_ref must provide the Stack Apps API key")
		}
		if !validSite(typed.Site) || (typed.UserID != "" && !validID(typed.UserID)) {
			return invalidArgument("init", "account.settings.site is required and user_id, when set, must be a positive integer")
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
	apiKey := account.AppID
	if typed.APIKeyRef != "" {
		apiKey, err = resolveSecret(ctx, resolved.Secrets, typed.APIKeyRef, "client")
		if err != nil {
			return nil, err
		}
	}
	accessToken := apiKey
	hasToken := false
	if account.AccessTokenRef != "" {
		accessToken, err = resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		hasToken = true
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken}}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, &httpClient, tokens, "stackexchange", productName,
		queryAuthenticator{key: apiKey, includeToken: hasToken}, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, site: typed.Site, userID: typed.UserID, api: api,
		hasToken: hasToken, scopes: append([]string(nil), account.Approval.Scopes...),
		clock: resolved.Clock, userAgent: settings.UserAgent, backoff: make(map[string]time.Time),
	}, nil
}

// OAuth returns an OAuth 2.0 authorization-code helper. Stack Exchange does
// not issue refresh tokens; expired or revoked tokens require authorization again.
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
	if account.SecretRef != "" {
		var err error
		secret, err = resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
		if err != nil {
			return nil, err
		}
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: settings.TokenURL, UserAgent: settings.UserAgent, HTTPClient: &httpClient, Clock: options.Clock,
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
	if len(value) == 0 || len(value) > 100 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '.' && character != '-' {
			return false
		}
	}
	return true
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

func validUserAgent(value string) bool {
	return len(strings.TrimSpace(value)) > 0 && len(value) <= 256 && !strings.ContainsAny(value, "\r\n")
}

var _ socialhub.Adapter = (*Adapter)(nil)
