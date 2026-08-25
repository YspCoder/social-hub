// Package merchantapi implements the Google Merchant API v1 shopping-data
// surface used to prepare and observe Shopping Ads inventory.
package merchantapi

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
	adapterName      = "google/merchant-api-v1"
	platformName     = "google-merchant-center"
	productName      = "merchant-api"
	apiVersion       = "v1"
	defaultBaseURL   = "https://merchantapi.googleapis.com"
	defaultAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL  = "https://oauth2.googleapis.com/token"
	documentationURL = "https://developers.google.com/merchant/api/reference/rest"
	contentScope     = "https://www.googleapis.com/auth/content"
)

// AccountSettings binds one SDK account to one Merchant Center account.
type AccountSettings struct {
	MerchantAccountID string `json:"merchant_account_id" yaml:"merchant_account_id"`
}

// Adapter implements socialhub.Adapter for Google Merchant API v1.
type Adapter struct {
	mu      sync.RWMutex
	config  socialhub.AdapterConfig
	options socialhub.Options
	ready   bool
	closed  bool
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
		return invalidArgument("init", "product must be merchant-api")
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; Merchant API and Google OAuth use fixed official HTTPS origins")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is required for every Merchant API account")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id, token_store, and webhook settings are not used by this adapter")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validNumericID(typed.MerchantAccountID, 20) {
			return invalidArgument("init", "account.settings.merchant_account_id must be a positive numeric Merchant Center account ID")
		}
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	adapter.config, adapter.options, adapter.ready = config, resolved, true
	return nil
}

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	baseOptions := adapter.options
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	requestIDs := newRequestIDFilter(
		account.ClientID, account.SecretRef, account.AccessTokenRef, accessToken,
		string(accountID), typed.MerchantAccountID,
	)
	tokens := &staticTokenSource{
		token: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}, requestIDs: requestIDs,
	}
	api, err := transport.New(defaultBaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock, requestIDs))
	if err != nil {
		tokens.Close()
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, merchantAccountID: typed.MerchantAccountID, api: api,
		scopes: append([]string(nil), account.Approval.Scopes...), tokens: tokens, requestIDs: requestIDs,
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

// OAuth returns Google's web-server authorization-code and refresh helper.
func (adapter *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	options := adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(account.ClientID, 1024) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	requestIDs := newRequestIDFilter(
		account.ClientID, account.SecretRef, account.AccessTokenRef, secret,
		string(accountID), typed.MerchantAccountID,
	)
	oauth := &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret,
		HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock, requestIDs: requestIDs,
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
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, errCredentialResolution)
	}
	if !validOpaque(value, 16384) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	return &copy
}

type staticTokenSource struct {
	mu         sync.RWMutex
	token      socialhub.Token
	requestIDs *requestIDFilter
	closed     bool
}

func (source *staticTokenSource) Token(context.Context) (socialhub.Token, error) {
	source.mu.RLock()
	defer source.mu.RUnlock()
	if source.closed {
		return socialhub.Token{}, platformError("token", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(source.token.AccessToken, 16384) {
		return socialhub.Token{}, socialhub.ErrUnauthenticated
	}
	return source.token, nil
}

func (source *staticTokenSource) Close() {
	source.mu.Lock()
	source.token, source.closed = socialhub.Token{}, true
	if source.requestIDs != nil {
		source.requestIDs.clear()
	}
	source.mu.Unlock()
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.config, adapter.options = socialhub.AdapterConfig{}, socialhub.Options{}
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
