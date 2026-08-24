// Package jinghongads implements Huawei Jinghong Momentum Marketing API for
// accounts in the Chinese mainland. The overseas Petal Ads contract lives in a
// separate adapter.
package jinghongads

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "huawei/jinghong-marketing-api-v1"
	platformName     = "jinghongads"
	productName      = "marketing-api"
	apiVersion       = "v1/reports-v2"
	defaultBaseURL   = "https://ads.cloud.huawei.com"
	defaultAuthURL   = "https://oauth-login.cloud.huawei.com/oauth2/v2/authorize"
	defaultTokenURL  = "https://oauth-login.cloud.huawei.com/oauth2/v2/token"
	documentationURL = "https://developer.huawei.com/consumer/cn/doc/promotion/ads_api02-0000001058566534"
)

// AccountSettings optionally binds one social-hub account to one advertiser.
// Huawei requires the advertiser ID for manager/service-provider identities,
// identities linked to multiple advertisers, and all reporting calls.
type AccountSettings struct {
	AdvertiserID string `json:"advertiser_id,omitempty" yaml:"advertiser_id,omitempty"`
}

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
		return invalidArgument("init", "product must be marketing-api")
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter-level settings are not supported; the mainland API and OAuth origins are fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is required for every Jinghong account")
		}
		if (account.ClientID == "") != (account.SecretRef == "") {
			return invalidArgument("init", "client_id and secret_ref must be configured together when OAuth helpers are used")
		}
		if account.ClientID != "" && !validClientID(account.ClientID) {
			return invalidArgument("init", "client_id must be a decimal Huawei OAuth client ID")
		}
		if account.SecretRef != "" && !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref must be a valid secret reference")
		}
		if account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "app_id and token_store are not used by this adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by this adapter")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.AdvertiserID != "" && !validID(typed.AdvertiserID) {
			return invalidArgument("init", "account.settings.advertiser_id must be a decimal advertiser ID")
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	api, err := transport.New(defaultBaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock))
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, advertiserID: typed.AdvertiserID, api: api,
		scopes: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
	}, nil
}

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
	if !validClientID(account.ClientID) || !validOpaque(account.SecretRef, 4096) {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: defaultAuthURL,
		TokenURL: defaultTokenURL, HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Jinghong credential", err, reference, value)
	}
	if !validOpaque(value, 8192) {
		return "", authenticationError(operation, "resolved Jinghong credential is invalid", nil, reference, value)
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
