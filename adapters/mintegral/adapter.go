// Package mintegral integrates Mintegral AppGrowth Open API through the
// pinned github.com/jageros/mintegral-go SDK.
package mintegral

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "mintegral/appgrowth-open-api-v1"
	platformName     = "mintegral"
	productName      = "appgrowth-open-api"
	apiVersion       = "v1/report-v2"
	documentationURL = "https://helpcenter.mintegral.com/en/docs/api-integration"
)

// Settings controls optional API origins. Overrides are primarily for
// loopback contract verification; the upstream SDK requires HTTPS otherwise.
type Settings struct {
	APIBaseURL     string `json:"api_base_url,omitempty" yaml:"api_base_url,omitempty"`
	StorageBaseURL string `json:"storage_base_url,omitempty" yaml:"storage_base_url,omitempty"`
}

// Adapter implements socialhub.Adapter for Mintegral AppGrowth Open API.
type Adapter struct {
	mu       sync.RWMutex
	config   socialhub.AdapterConfig
	options  socialhub.Options
	settings Settings
	ready    bool
	closed   bool
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
		return invalidArgument("init", "product must be appgrowth-open-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	var settings Settings
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if _, err := mtg.NewClient(upstreamOptions(settings, resolved)...); err != nil {
		return mapError("init", err)
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 512) || strings.TrimSpace(account.SecretRef) == "" {
			return invalidArgument("init", "client_id (Access Key) and secret_ref (API Key) are required")
		}
		if account.AppID != "" || account.AccessTokenRef != "" || account.TokenStore != "" || len(account.Settings) > 0 {
			return invalidArgument("init", "configure only client_id and secret_ref for each Mintegral account")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Mintegral Open API")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth approval fields are not used by Mintegral Open API")
		}
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	adapter.config, adapter.options, adapter.settings, adapter.ready = config, resolved, settings, true
	return nil
}

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	baseOptions, settings := adapter.options, adapter.settings
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
	apiKey, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || !validOpaque(apiKey, 4_096) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	credentials, err := mtg.NewCredentials(account.ClientID, apiKey)
	if err != nil {
		return nil, mapError("client", err)
	}
	upstream := upstreamOptions(settings, resolved)
	upstream = append(upstream, mtg.WithDefaultCredentials(credentials))
	sdk, err := mtg.NewClient(upstream...)
	if err != nil {
		return nil, mapError("client", err)
	}
	return &Client{accountID: accountID, sdk: sdk}, nil
}

func upstreamOptions(settings Settings, options socialhub.Options) []mtg.ClientOption {
	result := []mtg.ClientOption{
		mtg.WithHTTPClient(cloneHTTPClient(options.HTTPClient)),
		mtg.WithClock(options.Clock),
	}
	if settings.APIBaseURL != "" {
		result = append(result, mtg.WithAPIBaseURL(settings.APIBaseURL))
	}
	if settings.StorageBaseURL != "" {
		result = append(result, mtg.WithStorageBaseURL(settings.StorageBaseURL))
	}
	return result
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	return &copy
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
