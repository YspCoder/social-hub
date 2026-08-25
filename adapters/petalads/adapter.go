// Package petalads implements the overseas Huawei Petal Ads Marketing API.
// It intentionally does not implement the separate mainland China contract.
package petalads

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
	adapterName      = "huawei/petal-ads-marketing-api-v1"
	platformName     = "petalads"
	productName      = "marketing-api"
	apiVersion       = "v1/reports-v2"
	defaultAuthURL   = "https://oauth-login.cloud.huawei.com/oauth2/v2/authorize"
	defaultTokenURL  = "https://oauth-login.cloud.huawei.com/oauth2/v2/token"
	documentationURL = "https://developer.huawei.com/consumer/cn/doc/promotion/marketing-api-0000001174557681"
)

// Region selects an overseas Petal Ads API origin. Mainland China is a
// separate product contract and is deliberately not a valid region here.
type Region string

const (
	RegionAsiaAfricaLatinAmerica Region = "asia-africa-latin-america"
	RegionEurope                 Region = "europe"
	RegionRussia                 Region = "russia"
)

var regionOrigins = map[Region]string{
	RegionAsiaAfricaLatinAmerica: "https://ads-dra.cloud.huawei.com",
	RegionEurope:                 "https://ads-dre.cloud.huawei.com",
	RegionRussia:                 "https://ads-drru.cloud.huawei.ru",
}

// AccountSettings binds one social-hub account to an overseas region and,
// optionally, one advertiser. AdvertiserID is required for manager accounts or
// Huawei identities associated with more than one advertiser.
type AccountSettings struct {
	Region       Region `json:"region" yaml:"region"`
	AdvertiserID string `json:"advertiser_id,omitempty" yaml:"advertiser_id,omitempty"`
}

// Adapter implements socialhub.Adapter for the overseas Petal Ads API.
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
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; Petal Ads API and Huawei OAuth origins are fixed")
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is invalid")
		}
		if (account.ClientID == "") != (account.SecretRef == "") {
			return invalidArgument("init", "client_id and secret_ref must be configured together when OAuth helpers are used")
		}
		if account.ClientID != "" && !validClientID(account.ClientID) {
			return invalidArgument("init", "client_id must be a decimal Huawei OAuth client ID")
		}
		if account.SecretRef != "" && !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref is invalid")
		}
		if account.AccessTokenRef == "" && account.ClientID == "" {
			return invalidArgument("init", "access_token_ref or OAuth client credentials are required")
		}
		if account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "app_id and token_store are not used by this adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by this adapter")
		}
		if account.Approval.AccountType != "" {
			return invalidArgument("init", "approval.account_type is not part of the overseas Petal Ads contract")
		}
		if !validApprovalScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval scopes must be unique documented Petal Ads OAuth scopes")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if regionOrigins[typed.Region] == "" {
			return invalidArgument("init", "account.settings.region must be asia-africa-latin-america, europe, or russia")
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
	if !validOpaque(account.AccessTokenRef, 4096) {
		return nil, invalidArgument("client", "access_token_ref is required for an API client")
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
	requestIDValues := []string{accessToken, account.ClientID, string(account.ID), typed.AdvertiserID}
	api, err := transport.New(
		regionOrigins[typed.Region], httpClient, tokens, platformName, productName,
		newHTTPErrorDecoder(resolved.Clock, requestIDValues...),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, advertiserID: typed.AdvertiserID, region: typed.Region,
		api: api, scopes: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
		requestIDValues: requestIDValues,
	}, nil
}

// OAuth returns the Huawei OAuth 2.0 authorization-code and refresh helper.
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
	if !validClientID(account.ClientID) || strings.TrimSpace(account.SecretRef) == "" {
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
	if err != nil || !validOpaque(value, 8192) {
		return "", &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			PlatformMessage: "configured credential could not be resolved",
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
