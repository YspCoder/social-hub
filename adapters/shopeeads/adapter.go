// Package shopeeads implements shop-scoped Shopee Ads API v2 read workflows.
package shopeeads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "shopee/ads-api-v2"
	platformName     = "shopee"
	productName      = "ads-api"
	apiVersion       = "v2"
	defaultBaseURL   = "https://partner.shopeemobile.com"
	authorizePath    = "/api/v2/shop/auth_partner"
	documentationURL = "https://open.shopee.com/documents"
)

// Settings selects one official regional Shopee Open Platform origin.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings binds one social-hub account to one Shopee shop.
type AccountSettings struct {
	ShopID          int64  `json:"shop_id" yaml:"shop_id"`
	RefreshTokenRef string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Shopee Ads API v2.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(ctx context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if ctx == nil {
		return invalidArgument("init", "context is required")
	}
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if config.Product != "" && config.Product != productName {
		return invalidArgument("init", "product must be ads-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validShopeeOrigin(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an official Shopee Open Platform HTTPS origin")
	}
	for _, account := range config.Accounts {
		if _, err := parsePartnerID(account.ClientID); err != nil {
			return invalidArgument("init", "client_id must contain a positive Shopee partner_id")
		}
		if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the Shopee Partner Key is required")
		}
		if account.AppID != "" {
			return invalidArgument("init", "app_id is not part of the Shopee Open Platform signature contract")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "Shopee push callbacks are outside this Ads adapter")
		}
		if !validApproval(account.Approval) {
			return invalidArgument("init", "approval must omit scopes and use an official Shopee Ads app type")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validShopID(typed.ShopID) {
			return invalidArgument("init", "account.settings.shop_id must be a positive Shopee shop ID")
		}
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is invalid")
		}
		if typed.RefreshTokenRef != "" && !validOpaque(typed.RefreshTokenRef, 4096) {
			return invalidArgument("init", "account.settings.refresh_token_ref is invalid")
		}
		staticToken := account.AccessTokenRef != ""
		managedToken := typed.RefreshTokenRef != ""
		if staticToken == managedToken {
			return invalidArgument("init", "configure exactly one of access_token_ref or account.settings.refresh_token_ref")
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
	if ctx == nil {
		return nil, invalidArgument("client", "context is required")
	}
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
	partnerID, err := parsePartnerID(account.ClientID)
	if err != nil {
		return nil, invalidArgument("client", "client_id must contain a positive Shopee partner_id")
	}
	partnerKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	var tokens closeableTokenSource
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &staticTokenSource{token: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	} else {
		if resolved.TokenStore == nil {
			return nil, invalidArgument("client", "managed single-use refresh tokens require a token store")
		}
		refreshToken, err := resolveSecret(ctx, resolved.Secrets, typed.RefreshTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &refreshTokenSource{
			oauth: OAuthClient{
				PartnerID: partnerID, PartnerKey: partnerKey, BaseURL: settings.BaseURL,
				AuthURL: settings.BaseURL + authorizePath, HTTPClient: httpClient, Clock: resolved.Clock,
			},
			shopID: typed.ShopID, refreshToken: refreshToken, store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: tokenKeyPart(account.ClientID),
				Account: tokenKeyPart(string(accountID)), Subject: tokenKeyPart(formatID(typed.ShopID)),
			},
		}
	}
	signer := &shopAuthenticator{
		partnerID: partnerID, partnerKey: partnerKey, shopID: typed.ShopID, clock: resolved.Clock,
	}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, httpClient, tokens, platformName, productName,
		signer, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		tokens.Close()
		signer.Close()
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, shopID: typed.ShopID, api: api,
		approval: account.Approval, clock: resolved.Clock, tokens: tokens, signer: signer,
	}
	adapter.mu.RLock()
	available := adapter.ready && !adapter.closed
	adapter.mu.RUnlock()
	if !available {
		_ = client.Close()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return client, nil
}

// OAuth returns the Shopee seller authorization-code and token helper.
func (adapter *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	if ctx == nil {
		return nil, invalidArgument("oauth", "context is required")
	}
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
	partnerID, err := parsePartnerID(account.ClientID)
	if err != nil {
		return nil, invalidArgument("oauth", "client_id must contain a positive Shopee partner_id")
	}
	partnerKey, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	oauth := &OAuthClient{
		PartnerID: partnerID, PartnerKey: partnerKey, BaseURL: settings.BaseURL,
		AuthURL: settings.BaseURL + authorizePath, HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
	}
	adapter.mu.RLock()
	available := adapter.ready && !adapter.closed
	adapter.mu.RUnlock()
	if !available {
		oauth.Close()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return oauth, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	if ctx == nil {
		return "", invalidArgument(operation, "context is required")
	}
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", credentialError(operation)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	return &copy
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	adapter.config, adapter.options, adapter.settings = socialhub.AdapterConfig{}, socialhub.Options{}, Settings{}
	return nil
}

func tokenKeyPart(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:16])
}

func validApproval(approval socialhub.ApprovalConfig) bool {
	if len(approval.Scopes) != 0 {
		return false
	}
	switch approval.AccountType {
	case "", "seller-in-house-system", "marketing", "ads-service", "ads-service-app", "ads-facil":
		return true
	default:
		return false
	}
}

var _ socialhub.Adapter = (*Adapter)(nil)
