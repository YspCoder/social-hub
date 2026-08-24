// Package wechatminiprogram implements selected WeChat Mini Program server APIs.
package wechatminiprogram

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName                  = "wechat/mini-program"
	platformName                 = "wechat"
	productName                  = "mini-program"
	apiVersion                   = "continuous"
	baseURL                      = "https://api.weixin.qq.com"
	documentationURL             = "https://developers.weixin.qq.com/miniprogram/dev/server/API/"
	code2SessionDocumentationURL = "https://developers.weixin.qq.com/miniprogram/dev/server/API/user-login/api_code2session.html"
	stableTokenDocumentationURL  = "https://developers.weixin.qq.com/miniprogram/dev/server/API/mp-access-token/api_getstableaccesstoken.html"
	subscriptionDocumentationURL = "https://developers.weixin.qq.com/miniprogram/dev/server/API/mp-message-management/subscribe-message/api_sendmessage.html"
	phoneNumberDocumentationURL  = "https://developers.weixin.qq.com/miniprogram/dev/server/API/user-info/phone-number/api_getphonenumber.html"
)

// Adapter implements socialhub.Adapter for one or more Mini Program AppIDs.
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
		return invalidArgument("init", "product must be mini-program")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter settings are not supported; the official WeChat API origin is fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validSensitive(account.AppID, maxAppIDLength) {
			return invalidArgument("init", "account.app_id is required and invalid")
		}
		if !validSensitive(account.SecretRef, maxSecretReferenceLength) {
			return invalidArgument("init", "account.secret_ref is required")
		}
		if account.ClientID != "" || account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, access_token_ref, and token_store are not used; this adapter owns stable-token retrieval")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are outside this adapter")
		}
		if len(account.Settings) != 0 {
			return invalidArgument("init", "account settings are not supported")
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
	secret, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || !validSensitive(secret, maxCredentialLength) {
		return nil, authenticationError("client", err)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	return &Client{
		accountID:  accountID,
		appID:      account.AppID,
		appSecret:  secret,
		httpClient: &httpClient,
		clock:      resolved.Clock,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
