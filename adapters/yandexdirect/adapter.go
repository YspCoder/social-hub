// Package yandexdirect implements advertiser-scoped Yandex Direct API v5
// workflows through the production v501 JSON service endpoints.
package yandexdirect

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
	adapterName       = "yandex/direct-api-v5"
	platformName      = "yandex"
	productName       = "direct-api"
	apiVersion        = "v5"
	defaultBaseURL    = "https://api.direct.yandex.com/json/v501"
	defaultReportsURL = "https://api.direct.yandex.com/json/v501"
	sandboxBaseURL    = "https://api-sandbox.direct.yandex.com/json/v5"
	documentationURL  = "https://yandex.com/dev/direct/doc/en/"
	directScope       = "direct:api"
)

// Settings selects the official production or Sandbox API pair and the
// language used for provider messages.
type Settings struct {
	BaseURL        string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	ReportsBaseURL string `json:"reports_base_url,omitempty" yaml:"reports_base_url,omitempty"`
	AcceptLanguage string `json:"accept_language,omitempty" yaml:"accept_language,omitempty"`
}

// AccountSettings binds one social-hub account to one advertiser boundary.
// ClientLogin is required only when the OAuth token belongs to an agency.
type AccountSettings struct {
	ClientLogin      string `json:"client_login,omitempty" yaml:"client_login,omitempty"`
	UseOperatorUnits bool   `json:"use_operator_units,omitempty" yaml:"use_operator_units,omitempty"`
}

// Adapter implements socialhub.Adapter for Yandex Direct API v5.
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
		return invalidArgument("init", "product must be direct-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{
		BaseURL: defaultBaseURL, ReportsBaseURL: defaultReportsURL, AcceptLanguage: "en",
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validOfficialOrigins(settings.BaseURL, settings.ReportsBaseURL) {
		return invalidArgument("init", "base URLs must both select the official production v501 or Sandbox v5 origin")
	}
	if !validLanguage(settings.AcceptLanguage) {
		return invalidArgument("init", "accept_language must be en, ru, tr, or uk")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) || account.AccessTokenRef != strings.TrimSpace(account.AccessTokenRef) {
			return invalidArgument("init", "access_token_ref is required for every Yandex Direct account")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "configure only access_token_ref and account settings; Yandex OAuth token acquisition is external")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Yandex Direct API")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.ClientLogin != "" && !validLogin(typed.ClientLogin) {
			return invalidArgument("init", "account.settings.client_login is invalid")
		}
		if typed.UseOperatorUnits && typed.ClientLogin == "" {
			return invalidArgument("init", "use_operator_units requires an agency client_login")
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
	token, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: token, TokenType: "Bearer"}}
	requestIDValues := []string{token, account.AccessTokenRef, string(account.ID), typed.ClientLogin}
	api, err := transport.New(
		settings.BaseURL, httpClient, tokens, platformName, productName,
		newHTTPErrorDecoder(resolved.Clock, requestIDValues...),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	reportsAPI, err := transport.New(
		settings.ReportsBaseURL, httpClient, tokens, platformName, productName,
		newHTTPErrorDecoder(resolved.Clock, requestIDValues...),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, reportsAPI: reportsAPI, httpClient: httpClient,
		clientLogin: typed.ClientLogin, useOperatorUnits: typed.UseOperatorUnits,
		acceptLanguage: settings.AcceptLanguage, scopes: append([]string(nil), account.Approval.Scopes...),
		clock: resolved.Clock, requestIDValues: requestIDValues,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError("client", "could not resolve the Yandex OAuth token", err, reference, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError("client", "resolved Yandex OAuth token is invalid", nil, reference, value)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	return &copy
}

func validOfficialOrigins(baseURL, reportsURL string) bool {
	return baseURL == defaultBaseURL && reportsURL == defaultReportsURL ||
		baseURL == sandboxBaseURL && reportsURL == sandboxBaseURL
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
