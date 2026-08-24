// Package admanager implements network-scoped Google Ad Manager API v1 workflows.
// Publisher inventory and delivery resources remain separate from organic social interfaces.
package admanager

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
	adapterName       = "google/ad-manager-api-v1"
	platformName      = "google-ad-manager"
	productName       = "ad-manager-api"
	apiVersion        = "v1"
	discoveryRevision = "20260806"
	defaultBaseURL    = "https://admanager.googleapis.com"
	defaultAuthURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL   = "https://oauth2.googleapis.com/token"
	documentationURL  = "https://developers.google.com/ad-manager/api/beta/reference/rest"
	fullScope         = "https://www.googleapis.com/auth/admanager"
	readOnlyScope     = "https://www.googleapis.com/auth/admanager.readonly"
)

// Settings controls API and OAuth endpoints. Overrides are intended for tests
// and controlled HTTP gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings binds one social-hub account to one Ad Manager network.
type AccountSettings struct {
	NetworkCode     string `json:"network_code" yaml:"network_code"`
	RefreshTokenRef string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Google Ad Manager API v1.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be ad-manager-api")
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
		return invalidArgument("init", "Google endpoints must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validID(typed.NetworkCode) {
			return invalidArgument("init", "account settings.network_code must be a positive string-encoded integer")
		}
		staticToken := strings.TrimSpace(account.AccessTokenRef) != ""
		managedOAuth := !staticToken && validOpaque(account.ClientID, 1024) &&
			strings.TrimSpace(account.SecretRef) != "" && strings.TrimSpace(typed.RefreshTokenRef) != ""
		if !staticToken && !managedOAuth {
			return invalidArgument("init", "configure access_token_ref or client_id, secret_ref, and account.settings.refresh_token_ref")
		}
		if staticToken {
			if typed.RefreshTokenRef != "" {
				return invalidArgument("init", "refresh_token_ref cannot be combined with access_token_ref")
			}
			if (account.ClientID == "") != (account.SecretRef == "") {
				return invalidArgument("init", "client_id and secret_ref must be configured together")
			}
		}
		if managedOAuth && len(oauthScopes(account.Approval.Scopes)) == 0 {
			return invalidArgument("init", "approval scopes must include an Ad Manager API scope")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Google Ad Manager API")
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
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	clientScopes := append([]string(nil), account.Approval.Scopes...)
	var tokens socialhub.TokenSource
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{
			AccessToken: accessToken, TokenType: "Bearer", Scopes: clientScopes,
		}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		refreshToken, err := resolveSecret(ctx, resolved.Secrets, typed.RefreshTokenRef, "client")
		if err != nil {
			return nil, err
		}
		clientScopes = oauthScopes(account.Approval.Scopes)
		tokens = &refreshTokenSource{
			oauth: OAuthClient{
				ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
				TokenURL: settings.TokenURL, HTTPClient: httpClient, Clock: resolved.Clock, Scopes: clientScopes,
			},
			refreshToken: refreshToken,
			store:        resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Scopes: strings.Join(clientScopes, " "),
			},
		}
	}
	api, err := transport.New(settings.BaseURL, httpClient, tokens, platformName, productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, networkCode: typed.NetworkCode, api: api,
		scopes: clientScopes,
	}, nil
}

// OAuth returns Google's authorization-code and refresh-token helper.
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
	if !validOpaque(account.ClientID, 1024) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: settings.TokenURL, HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
		Scopes: oauthScopes(account.Approval.Scopes),
	}, nil
}

func oauthScopes(configured []string) []string {
	if len(configured) == 0 {
		return []string{readOnlyScope}
	}
	result := make([]string, 0, 2)
	for _, candidate := range []string{fullScope, readOnlyScope} {
		for _, scope := range configured {
			if scope == candidate {
				result = append(result, candidate)
				break
			}
		}
	}
	return result
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16384) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
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
