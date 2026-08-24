// Package kochava implements Kochava's marketer server-to-server measurement integration.
package kochava

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "kochava/s2s-measurement"
	platformName     = "kochava"
	productName      = "s2s-measurement"
	apiVersion       = "unversioned"
	defaultBaseURL   = "https://control.kochava.com"
	documentationURL = "https://support.kochava.com/articles/server-to-server-integration/388-server-to-server-integration-overview/"

	AccountTypePaid = "paid"
)

type accountSettings struct{}

// Adapter implements socialhub.Adapter for Kochava S2S measurement.
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
		return invalidArgument("init", "product must be s2s-measurement")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter-level settings are not supported; the Kochava ingestion origin is fixed")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AppID, 4096) {
			return invalidArgument("init", "app_id must contain the Kochava App GUID")
		}
		apiKeyConfigured := account.AccessTokenRef != ""
		secretConfigured := account.SecretRef != ""
		if apiKeyConfigured != secretConfigured {
			return invalidArgument("init", "strict authentication requires both access_token_ref (API Key) and secret_ref (app secret)")
		}
		if apiKeyConfigured && (!validOpaque(account.AccessTokenRef, 4096) || !validOpaque(account.SecretRef, 4096)) {
			return invalidArgument("init", "strict authentication credential references are invalid")
		}
		if account.Approval.AccountType != "" && account.Approval.AccountType != AccountTypePaid {
			return invalidArgument("init", "approval.account_type must be paid when configured")
		}
		if account.ClientID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "OAuth client, token store, webhook, and approval scopes are not used by this adapter")
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
	apiKey, appSecret := "", ""
	if account.AccessTokenRef != "" {
		apiKey, err = resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef)
		if err != nil {
			return nil, err
		}
		appSecret, err = resolveSecret(ctx, resolved.Secrets, account.SecretRef)
		if err != nil {
			return nil, err
		}
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{}}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, cloneHTTPClient(resolved.HTTPClient), tokens, platformName, productName,
		noAuthenticationHeader{}, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, appGUID: account.AppID, apiKey: apiKey, appSecret: appSecret,
		paid: account.Approval.AccountType == AccountTypePaid, api: api,
	}, nil
}

type noAuthenticationHeader struct{}

func (noAuthenticationHeader) Authenticate(*http.Request, socialhub.Token) error { return nil }

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError("client", "could not resolve a Kochava Strict Authentication credential")
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError("client", "resolved Kochava Strict Authentication credential is invalid")
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
