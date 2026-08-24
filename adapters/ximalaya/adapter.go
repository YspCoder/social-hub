// Package ximalaya implements a bounded, read-only Ximalaya Open Platform API adapter.
package ximalaya

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "ximalaya/open-api-v1"
	platformName     = "ximalaya"
	productName      = "open-api"
	apiVersion       = "1.0.0"
	defaultBaseURL   = "https://api.ximalaya.com"
	documentationURL = "https://open.ximalaya.com/api-docs"
	applicationURL   = "https://open.ximalaya.com/api-docs/document?id=107"
)

// AccountSettings contains the device identity required by the common API
// parameters. Values must describe the real calling device or server.
type AccountSettings struct {
	ClientOSType int          `json:"client_os_type" yaml:"client_os_type"`
	DeviceID     string       `json:"device_id" yaml:"device_id"`
	DeviceIDType DeviceIDType `json:"device_id_type" yaml:"device_id_type"`
}

// Adapter implements socialhub.Adapter for Ximalaya Open Platform API v1.
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
		return invalidArgument("init", "product must be open-api")
	}
	if len(config.Settings) > 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validCredential(account.ClientID) {
			return invalidArgument("init", "account.client_id must contain the Ximalaya app_key")
		}
		if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "account.secret_ref must reference the Ximalaya app_secret")
		}
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "account.access_token_ref must reference serverAuthenticateStaticKey")
		}
		if account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "app_id and token_store are outside this server-signature contract")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are outside this read-only adapter")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth approval metadata is outside this server-signature contract")
		}
		var settings AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validAccountSettings(settings) {
			return invalidArgument("init", "account settings require a valid client_os_type, device_id, and device_id_type")
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
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	appSecret, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil {
		return nil, authenticationError("client", err, appSecret)
	}
	if !validCredential(appSecret) {
		return nil, authenticationError("client", nil, appSecret)
	}
	staticKey, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil {
		return nil, authenticationError("client", err, appSecret, staticKey)
	}
	if !validCredential(staticKey) {
		return nil, authenticationError("client", nil, appSecret, staticKey)
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	redactions := []string{account.ClientID, appSecret, staticKey, settings.DeviceID}
	authenticator := &requestSigner{
		appKey: account.ClientID, signingKey: []byte(appSecret + staticKey),
		clientOSType: settings.ClientOSType, deviceID: settings.DeviceID,
		deviceIDType: settings.DeviceIDType, clock: resolved.Clock,
	}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "signed-request"}},
		platformName, productName, authenticator,
		newHTTPErrorDecoder(resolved.Clock, redactions),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, clock: resolved.Clock,
		secrets: []string{appSecret, staticKey}, redactions: redactions,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
