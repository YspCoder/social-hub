// Package aliexpressaffiliate implements AliExpress Affiliate publisher
// workflows. Affiliate products and attributed orders intentionally remain
// separate from social-hub's organic post model.
package aliexpressaffiliate

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "aliexpress/affiliate-api-v2"
	platformName     = "aliexpress"
	productName      = "affiliate-api"
	apiVersion       = "2.0"
	defaultBaseURL   = "https://api-sg.aliexpress.com"
	defaultPath      = "/sync"
	documentationURL = "https://open.aliexpress.com/doc/api.htm#/api?cid=21407"
	approvalURL      = "https://open.aliexpress.com/doc/doc.htm#/?docId=503"
	publisherType    = "aliexpress-affiliate-publisher"
)

// AccountSettings contains publisher-specific Affiliate parameters.
type AccountSettings struct {
	DefaultTrackingID   string `json:"default_tracking_id,omitempty" yaml:"default_tracking_id,omitempty"`
	DefaultAppSignature string `json:"default_app_signature,omitempty" yaml:"default_app_signature,omitempty"`
}

// Adapter implements socialhub.Adapter for AliExpress Affiliate API v2.
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
		return invalidArgument("init", "product must be affiliate-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 256) {
			return invalidArgument("init", "client_id must contain the AliExpress app_key")
		}
		if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the AliExpress app_secret is required")
		}
		if account.AccessTokenRef != "" {
			return invalidArgument("init", "access_token_ref is not used by the app-authenticated Affiliate methods in this adapter")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id, token_store, and webhook settings are not used by this adapter")
		}
		if (account.Approval.AccountType != "" && account.Approval.AccountType != publisherType) || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "approval.account_type may only be aliexpress-affiliate-publisher and OAuth scopes are not used")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.DefaultTrackingID != "" && !validCSVValue(typed.DefaultTrackingID, 512) {
			return invalidArgument("init", "account.settings.default_tracking_id is invalid")
		}
		if typed.DefaultAppSignature != "" && !validOpaque(typed.DefaultAppSignature, 4096) {
			return invalidArgument("init", "account.settings.default_app_signature is invalid")
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
	appSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	authenticator := &topAuthenticator{appKey: account.ClientID, clock: resolved.Clock}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, cloneHTTPClient(resolved.HTTPClient),
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: appSecret}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, defaultTrackingID: typed.DefaultTrackingID,
		defaultAppSignature: typed.DefaultAppSignature, gatewayPath: defaultPath,
		api: api, approval: account.Approval, clock: resolved.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the AliExpress app secret", err, reference, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved AliExpress app secret is invalid", nil, reference, value)
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
