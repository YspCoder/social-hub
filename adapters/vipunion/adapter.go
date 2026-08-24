// Package vipunion implements Vipshop Union Open API V2 affiliate workflows.
// Affiliate goods and attributed orders intentionally remain separate from
// social-hub's organic post model.
package vipunion

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "vipshop/union-open-api-v2"
	platformName     = "vipshop"
	productName      = "union-open-api"
	apiVersion       = "2.0.0"
	apiBaseURL       = "https://vop.vipapis.com"
	documentationURL = "https://vop.vip.com/home#/api/method/detail/com.vip.adp.api.open.service.UnionGoodsV2Service-2.0.0/queryWithOauth"
	approvalURL      = "https://vop.vip.com/home#/console/app/permission"
	publisherType    = "vipshop-union-publisher"
)

// AccountSettings contains publisher-specific Vipshop Union defaults.
type AccountSettings struct {
	DefaultChanTag string `json:"default_chan_tag,omitempty" yaml:"default_chan_tag,omitempty"`
	DefaultOpenID  string `json:"default_open_id,omitempty" yaml:"default_open_id,omitempty"`
	DefaultAdCode  string `json:"default_ad_code,omitempty" yaml:"default_ad_code,omitempty"`
}

// Adapter implements socialhub.Adapter for Vipshop Union Open API V2.
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
		return invalidArgument("init", "product must be union-open-api")
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
			return invalidArgument("init", "client_id must contain the Vipshop appKey")
		}
		if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the Vipshop appSecret is required")
		}
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref for the OAuth accessToken is required")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id, token_store, and webhook settings are not used by this adapter")
		}
		if (account.Approval.AccountType != "" && account.Approval.AccountType != publisherType) || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "approval.account_type may only be vipshop-union-publisher and OAuth scopes are not used")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.DefaultChanTag != "" && !validChanTag(typed.DefaultChanTag) {
			return invalidArgument("init", "account.settings.default_chan_tag is invalid")
		}
		if typed.DefaultOpenID != "" && !validOpenID(typed.DefaultOpenID) {
			return invalidArgument("init", "account.settings.default_open_id is invalid")
		}
		if typed.DefaultAdCode != "" && !validIdentifier(typed.DefaultAdCode, 64) {
			return invalidArgument("init", "account.settings.default_ad_code is invalid")
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	authenticator := &vipAuthenticator{
		appKey: account.ClientID, accessToken: accessToken, clock: resolved.Clock,
	}
	api, err := transport.NewWithAuthenticator(
		apiBaseURL, cloneHTTPClient(resolved.HTTPClient),
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: appSecret}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, defaultChanTag: typed.DefaultChanTag,
		defaultOpenID: typed.DefaultOpenID, defaultAdCode: typed.DefaultAdCode,
		api: api, approval: account.Approval, clock: resolved.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Vipshop credential", err, reference, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved Vipshop credential is invalid", nil, reference, value)
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
