// Package dailymotion implements the official Dailymotion API v2.
package dailymotion

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
	adapterName     = "dailymotion/api-v2"
	productName     = "dailymotion-api"
	apiVersion      = "v2"
	defaultBaseURL  = "https://api.dailymotion.com/v2"
	defaultTokenURL = "https://oauth2.dailymotion.com/v2/token"
	docURL          = "https://developers.dailymotion.com/reference/introduction"
)

// Settings controls Dailymotion API and OAuth endpoints. Overrides are useful
// for deterministic tests and approved gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings identifies the default managed profile for common fetches.
type AccountSettings struct {
	ProfileID string `json:"profile_id,omitempty" yaml:"profile_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Dailymotion API v2.
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
		Name: adapterName, Product: productName, APIVersion: apiVersion, DocURL: docURL,
		VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
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
	settings := Settings{BaseURL: defaultBaseURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.TokenURL) {
		return invalidArgument("init", "base_url and token_url must be absolute HTTP(S) URLs without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		usingStaticToken := strings.TrimSpace(account.AccessTokenRef) != ""
		usingCredentials := strings.TrimSpace(account.ClientID) != "" && strings.TrimSpace(account.SecretRef) != ""
		if !usingStaticToken && !usingCredentials {
			return invalidArgument("init", "access_token_ref or client_id with secret_ref is required for every account")
		}
		if len(account.Approval.Scopes) == 0 || !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "every account requires valid, non-duplicate Dailymotion scopes")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.ProfileID != "" && !validResourceID(typed.ProfileID) {
			return invalidArgument("init", "account.settings.profile_id is invalid")
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

	var tokens socialhub.TokenSource
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer", Scopes: append([]string(nil), account.Approval.Scopes...)}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &clientTokenSource{
			oauth:  OAuthClient{ClientID: account.ClientID, ClientSecret: secret, TokenURL: settings.TokenURL, HTTPClient: resolved.HTTPClient, Clock: resolved.Clock},
			scopes: append([]string(nil), account.Approval.Scopes...), store: resolved.TokenStore,
			key: socialhub.TokenKey{Platform: "dailymotion", Product: productName, Tenant: account.ClientID, Account: string(accountID), Scopes: strings.Join(account.Approval.Scopes, " ")},
		}
	}
	api, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "dailymotion", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	client := &Client{
		accountID: accountID, profileID: typed.ProfileID, scopes: append([]string(nil), account.Approval.Scopes...),
		api: api, apiBaseURL: baseURL, httpClient: resolved.HTTPClient, uploads: make(map[string]*videoUpload),
	}
	client.upload = &VideoUploadService{client: client}
	return client, nil
}

// OAuth returns a Dailymotion OAuth 2.0 Client Credentials helper.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	settings, options := a.settings, a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if strings.TrimSpace(account.ClientID) == "" || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{ClientID: account.ClientID, ClientSecret: secret, TokenURL: settings.TokenURL, HTTPClient: options.HTTPClient, Clock: options.Clock}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validResourceID(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= 256 && !strings.ContainsAny(value, "/?#")
}

var _ socialhub.Adapter = (*Adapter)(nil)
