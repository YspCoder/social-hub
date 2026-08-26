// Package singular implements Singular's SDID-based S2S EVENT API v2.
package singular

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "singular/s2s-events-v2"
	platformName     = "singular"
	productName      = "s2s-events"
	apiVersion       = "v2"
	defaultBaseURL   = "https://s2s.singular.net"
	documentationURL = "https://support.singular.net/hc/en-us/articles/31496864868635-Server-to-Server-EVENT-Endpoint-API-Reference"
)

type accountSettings struct{}

// Adapter implements socialhub.Adapter for Singular S2S EVENT v2.
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
		return invalidArgument("init", "product must be s2s-events")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter-level settings are not supported; the Singular S2S origin is fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is required for every Singular app")
		}
		if !validOpaque(account.AppID, 1024) {
			return invalidArgument("init", "app_id is required for every Singular app")
		}
		if account.ClientID != "" || account.SecretRef != "" || account.TokenStore != "" ||
			account.Webhook != (socialhub.WebhookConfig{}) || account.Approval.AccountType != "" || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "OAuth, token store, webhook, and approval settings are not used by this adapter")
		}
		var typed accountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
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
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: sdkKey}}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, cloneHTTPClient(resolved.HTTPClient), tokens, platformName, productName,
		noAuthenticationHeader{}, newHTTPErrorDecoder(resolved.Clock, sdkKey, account.AppID),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, appID: account.AppID, sdkKey: sdkKey, api: api,
	}, nil
}

type noAuthenticationHeader struct{}

func (noAuthenticationHeader) Authenticate(*http.Request, socialhub.Token) error { return nil }

func resolveSDKKey(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError("client", "could not resolve the Singular SDK Key", err, reference, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError("client", "resolved Singular SDK Key is invalid", nil, reference, value)
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
