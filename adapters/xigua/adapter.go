// Package xigua implements the official Xigua OpenAPI adapter.
package xigua

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName         = "xigua/openapi"
	platformName        = "xigua"
	productName         = "openapi"
	apiVersion          = "continuous"
	defaultBaseURL      = "https://open.douyin.com"
	defaultOAuthBaseURL = "https://open-api.ixigua.com"
	documentationURL    = "https://open.douyin.com/platform/resource/docs/ability/content-management/xigua-publish-solution"
)

// Settings controls the separate Xigua business and OAuth origins.
type Settings struct {
	BaseURL      string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	OAuthBaseURL string `json:"oauth_base_url,omitempty" yaml:"oauth_base_url,omitempty"`
	AuthURL      string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
}

// AccountSettings identifies the app-scoped authorized Xigua user.
type AccountSettings struct {
	OpenID string `json:"open_id" yaml:"open_id"`
}

// Adapter implements socialhub.Adapter for Xigua OpenAPI.
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
	if config.Product != "" && config.Product != productName {
		return invalidArgument("init", "product must be openapi")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{
		BaseURL: defaultBaseURL, OAuthBaseURL: defaultOAuthBaseURL,
		AuthURL: defaultOAuthBaseURL + "/oauth/connect",
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.OAuthBaseURL) || !validEndpoint(settings.AuthURL) {
		return invalidArgument("init", "base_url, oauth_base_url, and auth_url must be absolute HTTP(S) URLs without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef == "" && (!validOpaque(account.ClientID, 512) || strings.TrimSpace(account.SecretRef) == "") {
			return invalidArgument("init", "access_token_ref or client_id plus secret_ref is required for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.OpenID != "" && !validOpaque(typed.OpenID, 512) {
			return invalidArgument("init", "account.settings.open_id is invalid")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by this adapter version")
		}
		if len(account.Approval.Scopes) > 0 && !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "account approval scopes are invalid")
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
	if strings.TrimSpace(account.AccessTokenRef) == "" {
		return nil, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: "client",
			PlatformMessage: "a user access_token_ref is required; client_token cannot call user APIs",
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validOpaque(typed.OpenID, 512) {
		return nil, invalidArgument("client", "account.settings.open_id is required for user APIs")
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
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || !validOpaque(accessToken, maxOpaqueLength) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("access-token", token.AccessToken)
		return nil
	})
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken}}
	api, err := transport.NewWithAuthenticator(settings.BaseURL, &httpClient, tokens, platformName, productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	identity, err := transport.NewWithAuthenticator(settings.OAuthBaseURL, &httpClient, tokens, platformName, productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, openID: typed.OpenID, api: api, identity: identity, clock: resolved.Clock,
		scopes: append([]string(nil), account.Approval.Scopes...), uploads: make(map[string]*uploadState),
		submitted: make(map[string]socialhub.PublishStatus),
	}
	client.videos = &VideoService{client: client}
	return client, nil
}

// OAuth returns authorization-code, refresh-token, and client-token helpers.
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
	if !validOpaque(account.ClientID, 512) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || !validOpaque(secret, maxOpaqueLength) {
		return nil, platformError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	return &OAuthClient{
		ClientKey: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenBaseURL: settings.OAuthBaseURL, HTTPClient: &httpClient, Clock: options.Clock,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
