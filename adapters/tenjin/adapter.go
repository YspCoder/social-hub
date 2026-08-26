// Package tenjin implements Tenjin's v0 server-to-server measurement APIs.
package tenjin

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "tenjin/s2s-api-v0"
	platformName     = "tenjin"
	productName      = "s2s-api"
	apiVersion       = "v0"
	defaultBaseURL   = "https://track.tenjin.com"
	documentationURL = "https://tenjin.com/docs/server-to-server-s2s-setup/"
)

// AccountSettings binds one Tenjin app to its platform-specific contract.
type AccountSettings struct {
	Platform  Platform `json:"platform" yaml:"platform"`
	GoogleAds bool     `json:"google_ads,omitempty" yaml:"google_ads,omitempty"`
	MetaAEM   bool     `json:"meta_aem,omitempty" yaml:"meta_aem,omitempty"`
}

// Adapter implements socialhub.Adapter for Tenjin S2S measurement.
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
		return invalidArgument("init", "product must be s2s-api")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter-level settings are not supported; the Tenjin ingestion origin is fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validBundleID(account.AppID) {
			return invalidArgument("init", "app_id must contain the Tenjin app bundle ID")
		}
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is required for the Tenjin SDK Key")
		}
		if account.ClientID != "" || account.SecretRef != "" || account.TokenStore != "" ||
			account.Webhook != (socialhub.WebhookConfig{}) || account.Approval.AccountType != "" || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "OAuth client, secret, token store, webhook, and approval settings are not used by this adapter")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validPlatform(typed.Platform) {
			return invalidArgument("init", "account.settings.platform must be ios, android, amazon, or android_other")
		}
		if typed.MetaAEM && typed.Platform != PlatformIOS {
			return invalidArgument("init", "account.settings.meta_aem is valid only for iOS apps")
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
	sdkKey, err := resolveSDKKey(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: sdkKey}}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, cloneHTTPClient(resolved.HTTPClient), tokens, platformName, productName,
		basicAuthenticator{}, newHTTPErrorDecoder(resolved.Clock, sdkKey, account.AppID),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, bundleID: account.AppID, platform: typed.Platform,
		googleAds: typed.GoogleAds, metaAEM: typed.MetaAEM,
		api: api,
	}, nil
}

type basicAuthenticator struct{}

func (basicAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if !validOpaque(token.AccessToken, 16_384) {
		return invalidArgument("authenticate", "Tenjin SDK Key is invalid")
	}
	request.SetBasicAuth(token.AccessToken, "")
	return nil
}

func resolveSDKKey(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError("client", "could not resolve the Tenjin SDK Key")
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError("client", "resolved Tenjin SDK Key is invalid")
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
