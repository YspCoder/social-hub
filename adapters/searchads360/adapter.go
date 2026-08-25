// Package searchads360 implements the Search Ads 360 Reporting API v0 REST
// surface. It is intentionally separate from Google Ads, DV360, and CM360.
package searchads360

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "google/search-ads-360-reporting-api-v0"
	platformName     = "google-search-ads-360"
	productName      = "reporting-api"
	apiVersion       = "v0"
	defaultBaseURL   = "https://searchads360.googleapis.com"
	defaultAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL  = "https://oauth2.googleapis.com/token"
	documentationURL = "https://developers.google.com/search-ads/reporting/api/reference/rest"
	reportingScope   = "https://www.googleapis.com/auth/doubleclicksearch"
)

// AccountSettings binds one SDK account to a Search Ads 360 customer. Set
// LoginCustomerID when a manager account accesses a sub-manager or client.
type AccountSettings struct {
	CustomerID      string `json:"customer_id" yaml:"customer_id"`
	LoginCustomerID string `json:"login_customer_id,omitempty" yaml:"login_customer_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Search Ads 360 Reporting API v0.
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
		return invalidArgument("init", "product must be reporting-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; Google API and OAuth origins are fixed")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is required for every Search Ads 360 account")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) || account.Approval.AccountType != "" {
			return invalidArgument("init", "app_id, token_store, webhook settings, and approval.account_type are not used by this adapter")
		}
		if (account.ClientID == "") != (account.SecretRef == "") || account.ClientID != "" &&
			(!validOpaque(account.ClientID, 1024) || !validOpaque(account.SecretRef, 4096)) {
			return invalidArgument("init", "client_id and secret_ref must be configured together for the optional OAuth helper")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validCustomerID(typed.CustomerID) {
			return invalidArgument("init", "account.settings.customer_id must contain exactly 10 ASCII digits without hyphens")
		}
		if typed.LoginCustomerID != "" && !validCustomerID(typed.LoginCustomerID) {
			return invalidArgument("init", "account.settings.login_customer_id must contain exactly 10 ASCII digits without hyphens")
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
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		if typed.LoginCustomerID != "" {
			request.Header.Set("login-customer-id", typed.LoginCustomerID)
		}
		return nil
	})
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, tokens, platformName, productName, authenticator,
		newHTTPErrorDecoder(resolved.Clock, accessToken, typed.CustomerID, typed.LoginCustomerID),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, customerID: typed.CustomerID, api: api,
		scopes: append([]string(nil), account.Approval.Scopes...),
	}, nil
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
	if !validOpaque(account.ClientID, 1024) || !validOpaque(account.SecretRef, 4096) {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		clientID: account.ClientID, clientSecret: secret,
		httpClient: cloneHTTPClient(options.HTTPClient), clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16384) {
		return "", &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			PlatformMessage: "configured Google credential could not be resolved", ApprovalURL: documentationURL,
		}
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
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
