// Package giphy implements the official GIPHY API.
package giphy

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName            = "giphy/v1"
	productName            = "api"
	apiVersion             = "v1"
	defaultBaseURL         = "https://api.giphy.com/v1"
	defaultUploadURL       = "https://upload.giphy.com/v1"
	defaultAnalyticsOrigin = "https://giphy-analytics.giphy.com"
	documentationURL       = "https://developers.giphy.com/docs/api/"
)

// Settings controls GIPHY API, upload, and analytics endpoints.
type Settings struct {
	BaseURL         string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	UploadURL       string `json:"upload_url,omitempty" yaml:"upload_url,omitempty"`
	AnalyticsOrigin string `json:"analytics_origin,omitempty" yaml:"analytics_origin,omitempty"`
}

// Adapter implements socialhub.Adapter for GIPHY API v1.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL, UploadURL: defaultUploadURL, AnalyticsOrigin: defaultAnalyticsOrigin}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.UploadURL) || !validOrigin(settings.AnalyticsOrigin) {
		return invalidArgument("init", "GIPHY endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment; analytics_origin must not contain a path")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 512) {
			return invalidArgument("init", "client_id must contain the GIPHY API key")
		}
		if len(account.Settings) > 0 {
			var empty struct{}
			if err := socialhub.DecodeSettings(account.Settings, &empty); err != nil {
				return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
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

func (adapter *Adapter) Client(_ context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
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
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: account.ClientID}}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, &httpClient, tokens, "giphy", productName,
		transport.QueryAuthenticator("api_key"), decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	upload, err := transport.NewWithAuthenticator(
		settings.UploadURL, &httpClient, tokens, "giphy", productName,
		transport.QueryAuthenticator("api_key"), decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	noAuth := transport.AuthenticatorFunc(func(*http.Request, socialhub.Token) error { return nil })
	analytics, err := transport.NewWithAuthenticator(
		settings.AnalyticsOrigin, &httpClient, socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "unused"}},
		"giphy", productName, noAuth, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, upload: upload, analytics: analytics,
		analyticsOrigin: normalizedOrigin(settings.AnalyticsOrigin), clock: resolved.Clock,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
