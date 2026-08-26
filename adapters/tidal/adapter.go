// Package tidal implements the read-only TIDAL Developer Platform API v2 catalog surface.
package tidal

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "tidal/api-v2"
	platformName     = "tidal"
	productName      = "tidal-api"
	apiVersion       = "v2"
	defaultBaseURL   = "https://openapi.tidal.com/v2"
	documentationURL = "https://tidal-music.github.io/tidal-api-reference/"
)

// Adapter implements socialhub.Adapter for the TIDAL API v2 catalog.
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
		return invalidArgument("init", "product must be tidal-api")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter settings are not supported; the official TIDAL API origin is fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, maxCredentialLength) {
			return invalidArgument("init", "account.access_token_ref is required")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" ||
			len(account.Settings) != 0 || account.Webhook != (socialhub.WebhookConfig{}) ||
			account.Approval.AccountType != "" || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "OAuth client, app, secret, token store, account, webhook, and approval settings are not used by this adapter")
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
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil {
		return nil, authenticationError("client", "could not resolve the TIDAL access token")
	}
	if !validOpaque(accessToken, maxCredentialLength) {
		return nil, authenticationError("client", "resolved TIDAL access token is invalid")
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	return &Client{
		accountID:   accountID,
		httpClient:  &httpClient,
		accessToken: accessToken,
		clock:       resolved.Clock,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
