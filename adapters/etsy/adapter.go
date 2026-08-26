// Package etsy implements bounded seller listing workflows for Etsy Open API v3.
package etsy

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "etsy/open-api-v3"
	platformName     = "etsy"
	productName      = "open-api"
	apiVersion       = "v3"
	defaultBaseURL   = "https://openapi.etsy.com"
	defaultAuthURL   = "https://www.etsy.com/oauth/connect"
	defaultTokenURL  = "https://openapi.etsy.com/v3/public/oauth/token"
	documentationURL = "https://developers.etsy.com/documentation/"
)

// Settings controls Etsy API and OAuth endpoints. Overrides are intended for
// controlled contract-verification gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings binds one social-hub account to an Etsy shop. A refresh
// token reference enables managed OAuth refresh; omit both token references
// for API-key-only access to public endpoints.
type AccountSettings struct {
	ShopID          int64  `json:"shop_id" yaml:"shop_id"`
	RefreshTokenRef string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Etsy Open API v3.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if config.Product != "" && config.Product != productName {
		return invalidArgument("init", "product must be open-api")
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
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.AuthURL) || !validEndpoint(settings.TokenURL) {
		return invalidArgument("init", "base_url, auth_url, and token_url must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 1024) || !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "client_id and secret_ref are required for the Etsy keystring and shared secret")
		}
		if account.AppID != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id and webhook settings are not used by these listing workflows")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.ShopID <= 0 {
			return invalidArgument("init", "account.settings.shop_id must be a positive integer")
		}
		staticToken := account.AccessTokenRef != ""
		managedRefresh := typed.RefreshTokenRef != ""
		if staticToken && managedRefresh {
			return invalidArgument("init", "access_token_ref cannot be combined with account.settings.refresh_token_ref")
		}
		if (staticToken && !validOpaque(account.AccessTokenRef, 4096)) ||
			(managedRefresh && !validOpaque(typed.RefreshTokenRef, 4096)) {
			return invalidArgument("init", "token secret references are invalid")
		}
		for _, scope := range account.Approval.Scopes {
			if !validOAuthScope(scope) {
				return invalidArgument("init", "approval scopes contain a scope not documented by Etsy Open API v3")
			}
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
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
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
	sharedSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	apiKey := account.ClientID + ":" + sharedSecret
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	var tokens socialhub.TokenSource = apiKeyOnlyTokenSource{}
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{
			AccessToken: accessToken, TokenType: "Bearer", Scopes: append([]string(nil), account.Approval.Scopes...),
		}}
	} else if typed.RefreshTokenRef != "" {
		refreshToken, err := resolveSecret(ctx, resolved.Secrets, typed.RefreshTokenRef, "client")
		if err != nil {
			return nil, err
		}
		scopes := normalizedScopes(account.Approval.Scopes)
		tokens = &refreshTokenSource{
			client: OAuthClient{
				ClientID: account.ClientID, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL,
				HTTPClient: httpClient, Clock: resolved.Clock,
			},
			refreshToken: refreshToken,
			store:        resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Subject: formatID(typed.ShopID), Scopes: strings.Join(scopes, " "),
			},
		}
	}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, httpClient, tokens, platformName, productName,
		credentialAuthenticator{APIKey: apiKey}, newHTTPErrorDecoder(resolved.Clock, apiKey, sharedSecret),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, shopID: typed.ShopID, api: api, approval: account.Approval,
		clock: resolved.Clock, hasOAuth: account.AccessTokenRef != "" || typed.RefreshTokenRef != "",
	}, nil
}

// OAuth returns an Etsy authorization-code, PKCE, and refresh helper.
func (adapter *Adapter) OAuth(accountID socialhub.AccountID) (*OAuthClient, error) {
	adapter.mu.RLock()
	defer adapter.mu.RUnlock()
	if !adapter.ready || adapter.closed {
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return &OAuthClient{
		ClientID: account.ClientID, AuthURL: adapter.settings.AuthURL, TokenURL: adapter.settings.TokenURL,
		HTTPClient: cloneHTTPClient(adapter.options.HTTPClient), Clock: adapter.options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func normalizedScopes(scopes []string) []string {
	result := append([]string(nil), scopes...)
	sort.Strings(result)
	return result
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
