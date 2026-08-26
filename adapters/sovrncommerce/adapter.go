// Package sovrncommerce implements verified Sovrn Commerce publisher
// affiliate and reporting workflows.
package sovrncommerce

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "sovrn/commerce-api-v20230630"
	platformName     = "sovrn"
	productName      = "commerce-api"
	apiVersion       = "Reports 20230630; Merchant Rates 1.0.0; affiliate links unversioned"
	documentationURL = "https://developer.sovrn.com/"

	defaultReportsBaseURL       = "https://viglink.io/v1"
	defaultMerchantRatesBaseURL = "https://viglink.io/merchants/rates"
	publisherAccountType        = "commerce-publisher"
)

// Adapter implements socialhub.Adapter for Sovrn Commerce publisher APIs.
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
		return invalidArgument("init", "product must be commerce-api")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter-level settings are not supported; Sovrn Commerce origins are fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 4096) || !validOpaque(account.SecretRef, 4096) ||
			account.AccessTokenRef != "" || account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id and secret_ref are required; access_token_ref, app_id, and token_store are not used")
		}
		if len(account.Settings) > 0 {
			return invalidArgument("init", "account settings are not used by this adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by these request/response workflows")
		}
		if (account.Approval.AccountType != "" && account.Approval.AccountType != publisherAccountType) || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "approval.account_type must be commerce-publisher and OAuth scopes are not used")
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
	secretKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: secretKey, TokenType: "secret"}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validOpaque(token.AccessToken, 16_384) {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("Authorization", "secret "+token.AccessToken)
		return nil
	})
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	decoder := newHTTPErrorDecoder(resolved.Clock, secretKey, account.ClientID)
	reportsAPI, err := transport.NewWithAuthenticator(
		defaultReportsBaseURL, httpClient, tokens, platformName, productName, authenticator, decoder,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	merchantRatesAPI, err := transport.NewWithAuthenticator(
		defaultMerchantRatesBaseURL, httpClient, tokens, platformName, productName, authenticator, decoder,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, apiKey: account.ClientID, reportsAPI: reportsAPI, merchantRatesAPI: merchantRatesAPI,
		approval: account.Approval, redactionSecrets: []string{secretKey, account.ClientID},
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Sovrn Commerce Secret Key", err, reference, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved Sovrn Commerce Secret Key is invalid", nil, reference, value)
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
