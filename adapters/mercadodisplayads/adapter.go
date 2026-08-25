// Package mercadodisplayads implements Mercado Libre Display Ads read and metrics workflows.
package mercadodisplayads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "mercadolibre/display-ads-api-v1"
	platformName     = "mercadolibre"
	productName      = "display-ads-api"
	apiVersion       = "v1"
	defaultBaseURL   = "https://api.mercadolibre.com"
	defaultAuthURL   = "https://auth.mercadolibre.com.ar/authorization"
	defaultTokenURL  = "https://api.mercadolibre.com/oauth/token"
	documentationURL = "https://developers.mercadolibre.com.ar/en_us/display-ads"
	approvalType     = "commercial-advisor-enabled"
)

// Settings selects the country-specific official authorization endpoint. API
// and token requests always use Mercado Libre's fixed official origin.
type Settings struct {
	AuthURL string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
}

// AccountSettings binds one SDK account to one Display Ads advertiser. A zero
// AdvertiserID permits advertiser discovery but not advertiser-scoped reads.
type AccountSettings struct {
	AdvertiserID    int64  `json:"advertiser_id,omitempty" yaml:"advertiser_id,omitempty"`
	RefreshTokenRef string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
}

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

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if config.Product != "" && config.Product != productName {
		return invalidArgument("init", "product must be display-ads-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{AuthURL: defaultAuthURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validAuthorizationEndpoint(settings.AuthURL) {
		return invalidArgument("init", "auth_url must be an official Mercado Libre HTTPS authorization endpoint")
	}
	for _, account := range config.Accounts {
		if account.AppID != "" {
			return invalidArgument("init", "app_id is not used; configure the Mercado Libre App ID in client_id")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "Mercado Libre notifications are outside this Display Ads adapter")
		}
		if account.ClientID != "" && !validPositiveDecimal(account.ClientID) {
			return invalidArgument("init", "client_id must be a positive decimal Mercado Libre App ID")
		}
		if (account.ClientID == "") != (account.SecretRef == "") {
			return invalidArgument("init", "client_id and secret_ref must be configured together")
		}
		if account.SecretRef != "" && !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref is invalid")
		}
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is invalid")
		}
		if !validApproval(account.Approval) {
			return invalidArgument("init", "approval must be omitted or declare commercial-advisor-enabled with the read scope")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.AdvertiserID < 0 {
			return invalidArgument("init", "account.settings.advertiser_id must be zero or a positive advertiser ID")
		}
		if typed.RefreshTokenRef != "" && !validOpaque(typed.RefreshTokenRef, 4096) {
			return invalidArgument("init", "account.settings.refresh_token_ref is invalid")
		}
		staticToken := account.AccessTokenRef != ""
		managedToken := typed.RefreshTokenRef != ""
		if staticToken == managedToken {
			return invalidArgument("init", "configure exactly one of access_token_ref or account.settings.refresh_token_ref")
		}
		if managedToken && account.ClientID == "" {
			return invalidArgument("init", "client_id and secret_ref are required for managed refresh tokens")
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
		clientSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		refreshToken, err := resolveSecret(ctx, resolved.Secrets, typed.RefreshTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &refreshTokenSource{
			oauth: OAuthClient{
				ClientID: account.ClientID, ClientSecret: clientSecret, AuthURL: settings.AuthURL,
				TokenURL: defaultTokenURL, HTTPClient: httpClient, Clock: resolved.Clock,
			},
			refreshToken: refreshToken, store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName,
				Tenant: tokenKeyPart(account.ClientID), Account: tokenKeyPart(string(accountID)),
			},
		}
	}
	api, err := transport.New(
		defaultBaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		tokens.Close()
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, advertiserID: typed.AdvertiserID, api: api,
		approval: account.Approval, tokens: tokens,
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

// OAuth returns the Mercado Libre authorization-code, optional PKCE, and token helper.
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
	if account.ClientID == "" || account.SecretRef == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	oauth := &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: defaultTokenURL, HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
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

func approvalConfigured(approval socialhub.ApprovalConfig) bool {
	return approval.AccountType == approvalType && containsScope(approval.Scopes, "read")
}

func validApproval(approval socialhub.ApprovalConfig) bool {
	if approval.AccountType == "" && len(approval.Scopes) == 0 {
		return true
	}
	if approval.AccountType != approvalType || !containsScope(approval.Scopes, "read") {
		return false
	}
	seen := make(map[string]struct{}, len(approval.Scopes))
	for _, scope := range approval.Scopes {
		if scope != strings.TrimSpace(scope) || scope != "offline_access" && scope != "read" && scope != "write" {
			return false
		}
		if _, duplicate := seen[scope]; duplicate {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func containsScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func tokenKeyPart(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:16])
}

var _ socialhub.Adapter = (*Adapter)(nil)
