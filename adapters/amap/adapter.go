// Package amap implements the current Amap Web Service Place Search v5 API.
package amap

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "amap/web-service-place-v5"
	platformName     = "amap"
	productName      = "web-service-place"
	apiVersion       = "v5"
	defaultBaseURL   = "https://restapi.amap.com"
	documentationURL = "https://lbs.amap.com/api/webservice/guide/api-advanced/newpoisearch"
	keyManagementURL = "https://lbs.amap.com/api/webservice/guide/create-project/get-key"
	termsURL         = "https://lbs.amap.com/home/terms/"
	quotaURL         = "https://lbs.amap.com/api/webservice/guide/tools/flowlevel"
)

// Adapter implements socialhub.Adapter for Amap Place Search v5.
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
		return invalidArgument("init", "product must be web-service-place")
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
		if !validReference(account.AccessTokenRef) {
			return invalidArgument("init", "account.access_token_ref must reference an Amap Web Service key")
		}
		if account.SecretRef != "" && !validReference(account.SecretRef) {
			return invalidArgument("init", "account.secret_ref must reference the optional digital-signature private key")
		}
		if account.ClientID != "" || account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, app_id, and token_store are outside this API-key contract")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are outside this read-only request/response API")
		}
		var empty struct{}
		if err := socialhub.DecodeSettings(account.Settings, &empty); err != nil {
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
	apiKey, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || !validCredential(apiKey) {
		return nil, authenticationError("client")
	}
	var signingSecret string
	if account.SecretRef != "" {
		signingSecret, err = resolved.Secrets.Resolve(ctx, account.SecretRef)
		if err != nil || !validCredential(signingSecret) {
			return nil, authenticationError("client")
		}
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	redactions := []string{apiKey, signingSecret}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: apiKey}},
		platformName, productName, &queryAuthenticator{signingSecret: signingSecret},
		newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{accountID: accountID, api: api, clock: resolved.Clock, secrets: redactions}
	adapter.mu.RLock()
	available := adapter.ready && !adapter.closed
	adapter.mu.RUnlock()
	if !available {
		_ = client.Close()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	return client, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	adapter.config = socialhub.AdapterConfig{}
	adapter.options = socialhub.Options{}
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
