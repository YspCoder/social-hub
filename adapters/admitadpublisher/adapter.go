// Package admitadpublisher implements Admitad Publisher API affiliate
// program, deeplink, coupon, and reporting workflows.
package admitadpublisher

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
	adapterName      = "admitad/publisher-api"
	platformName     = "admitad"
	productName      = "publisher-api"
	apiVersion       = "unversioned"
	defaultBaseURL   = "https://api.admitad.com"
	defaultTokenURL  = "https://api.admitad.com/token/"
	documentationURL = "https://developers.mitgo.com/hc/en-us/categories/34481291136402-Publisher-API"

	scopePrograms   = "advcampaigns_for_website"
	scopeDeeplinks  = "deeplink_generator"
	scopeCoupons    = "coupons_for_website"
	scopeStatistics = "statistics"
)

// Settings controls the Publisher API and OAuth endpoints. Overrides are
// intended for controlled contract-verification gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// Adapter implements socialhub.Adapter for the Admitad Publisher API.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be publisher-api")
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
	if !validBaseURL(settings.BaseURL) || !validEndpoint(settings.TokenURL) {
		return invalidArgument("init", "base_url and token_url must be absolute HTTP(S) URLs without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		staticToken := strings.TrimSpace(account.AccessTokenRef) != ""
		managedOAuth := validOpaque(account.ClientID, 1024) && strings.TrimSpace(account.SecretRef) != ""
		if staticToken == managedOAuth {
			return invalidArgument("init", "configure exactly one of access_token_ref or client_id with secret_ref")
		}
		if staticToken {
			if !validOpaque(account.AccessTokenRef, 4096) || account.ClientID != "" || account.SecretRef != "" {
				return invalidArgument("init", "access_token_ref cannot be combined with client_id or secret_ref")
			}
		} else if !validOpaque(account.SecretRef, 4096) || len(account.Approval.Scopes) == 0 {
			return invalidArgument("init", "managed OAuth requires secret_ref and at least one approval scope")
		}
		if account.AppID != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id and webhook settings are not used by this adapter")
		}
		if len(account.Settings) != 0 {
			return invalidArgument("init", "account.settings is not used; pass website IDs to individual workflows")
		}
		if !validOAuthScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval scopes contain an invalid or duplicate value")
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
	scopes := append([]string(nil), account.Approval.Scopes...)
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	var tokens socialhub.TokenSource
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{
			AccessToken: accessToken, TokenType: "Bearer", Scopes: scopes,
		}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &managedTokenSource{
			oauth: OAuthClient{
				ClientID: account.ClientID, ClientSecret: secret, TokenURL: settings.TokenURL,
				HTTPClient: httpClient, Clock: resolved.Clock, Scopes: scopes,
			},
			store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Scopes: strings.Join(scopes, " "),
			},
		}
	}
	api, err := transport.New(settings.BaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock))
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api, scopes: scopes}, nil
}

// OAuth returns the Client Credentials and refresh-token helper for a managed
// OAuth account.
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
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		return nil, invalidArgument("oauth", "managed OAuth is unavailable for a static-token account")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, TokenURL: settings.TokenURL,
		HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
		Scopes: append([]string(nil), account.Approval.Scopes...),
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
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
