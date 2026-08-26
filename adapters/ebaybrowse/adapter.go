// Package ebaybrowse implements eBay Buy Browse API v1 affiliate discovery workflows.
// Marketplace items and affiliate URLs intentionally remain separate from
// social-hub's organic post model.
package ebaybrowse

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
	adapterName      = "ebay/buy-browse-api-v1"
	platformName     = "ebay"
	productName      = "buy-browse-api"
	apiVersion       = "v1.20.4"
	defaultBaseURL   = "https://api.ebay.com/buy/browse/v1"
	defaultTokenURL  = "https://api.ebay.com/identity/v1/oauth2/token"
	documentationURL = "https://developer.ebay.com/api-docs/buy/browse/overview.html"
	applicationScope = "https://api.ebay.com/oauth/api_scope"
)

// Settings controls the Browse and OAuth endpoints. Overrides are intended
// for eBay Sandbox or a controlled contract-verification gateway.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings contains marketplace and eBay Partner Network defaults.
type AccountSettings struct {
	MarketplaceID       string `json:"marketplace_id,omitempty" yaml:"marketplace_id,omitempty"`
	AffiliateCampaignID string `json:"affiliate_campaign_id,omitempty" yaml:"affiliate_campaign_id,omitempty"`
	AcceptLanguage      string `json:"accept_language,omitempty" yaml:"accept_language,omitempty"`
}

// Adapter implements socialhub.Adapter for eBay Buy Browse API v1.
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
		return invalidArgument("init", "product must be buy-browse-api")
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
		return invalidArgument("init", "eBay endpoints must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
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
		} else if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the eBay Cert ID is required")
		}
		if account.AppID != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id and webhook settings are not used by this adapter")
		}
		for _, scope := range account.Approval.Scopes {
			if !validOpaque(scope, 2048) {
				return invalidArgument("init", "approval scopes contain an invalid value")
			}
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.MarketplaceID != "" && !validMarketplaceID(typed.MarketplaceID) {
			return invalidArgument("init", "account.settings.marketplace_id is invalid")
		}
		if typed.AffiliateCampaignID != "" && !validOpaque(typed.AffiliateCampaignID, 256) {
			return invalidArgument("init", "account.settings.affiliate_campaign_id is invalid")
		}
		if typed.AcceptLanguage != "" && !validLanguageTag(typed.AcceptLanguage) {
			return invalidArgument("init", "account.settings.accept_language is invalid")
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
	if typed.MarketplaceID == "" {
		typed.MarketplaceID = "EBAY_US"
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	var tokens socialhub.TokenSource
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{
			AccessToken: accessToken, TokenType: "Bearer", Scopes: append([]string(nil), account.Approval.Scopes...),
		}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &applicationTokenSource{
			oauth: OAuthClient{
				ClientID: account.ClientID, ClientSecret: secret, TokenURL: settings.TokenURL,
				HTTPClient: httpClient, Clock: resolved.Clock,
			},
			store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Scopes: applicationScope,
			},
		}
	}
	api, err := transport.New(settings.BaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock))
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, approval: account.Approval,
		marketplaceID: typed.MarketplaceID, affiliateCampaignID: typed.AffiliateCampaignID,
		acceptLanguage: typed.AcceptLanguage,
	}, nil
}

// OAuth returns the Client Credentials helper for a managed OAuth account.
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
