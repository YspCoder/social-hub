// Package applovinads implements AppLovin Axon Campaign Management API v1.
// Paid user-acquisition resources remain separate from social-hub's organic interfaces.
package applovinads

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "applovin/growth-campaign-management-api-v1"
	platformName     = "applovin"
	productName      = "growth-campaign-management-api"
	apiVersion       = "v1"
	contractVersion  = "verified-2026-08-25"
	apiBaseURL       = "https://api.ads.axon.ai/manage/v1"
	documentationURL = "https://support.applovin.com/en/growth/promoting-your-apps/api/axon-campaign-management-api"
	approvalURL      = "https://support.applovin.com/en/growth/introduction/applovin-mcp"
	approvalScope    = "campaign_management_api"
)

// AccountSettings binds one social-hub account to one Axon Ads Manager account.
type AccountSettings struct {
	AccountID   string      `json:"account_id" yaml:"account_id"`
	AccountType AccountType `json:"account_type" yaml:"account_type"`
}

// Adapter implements socialhub.Adapter for Axon Campaign Management API v1.
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
		return invalidArgument("init", "product must be growth-campaign-management-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; the Axon API endpoint is fixed")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validNumericID(typed.AccountID) || !validAccountType(typed.AccountType) {
			return invalidArgument("init", "account.settings requires a numeric account_id and account_type APP or WEB")
		}
		if !validOpaque(account.SecretRef, 4096) || account.ClientID != "" || account.AppID != "" ||
			account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "configure only secret_ref with the Campaign Management API key")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Campaign Management API v1")
		}
		if !validApproval(account.Approval.AccountType, account.Approval.Scopes) {
			return invalidArgument("init", "approval.account_type must be empty and approval.scopes may contain only campaign_management_api")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	apiKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: apiKey}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validOpaque(token.AccessToken, 16384) {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("Authorization", token.AccessToken)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		apiBaseURL, httpClient, tokens, platformName, productName, authenticator,
		func(status int, header http.Header, body []byte) error {
			return decodeHTTPError(status, header, body, resolved.Clock)
		},
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, axonAccountID: typed.AccountID, accountType: typed.AccountType,
		api: api, approved: contains(account.Approval.Scopes, approvalScope),
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the AppLovin Campaign Management API key")
	}
	if !validOpaque(value, 16384) {
		return "", authenticationError(operation, "resolved AppLovin Campaign Management API key is invalid")
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
