// Package adjust implements Adjust's server-to-server API.
package adjust

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "adjust/s2s-api"
	platformName     = "adjust"
	productName      = "s2s-api"
	apiVersion       = "unversioned"
	defaultBaseURL   = "https://s2s.adjust.com"
	documentationURL = "https://dev.adjust.com/en/api/s2s-api/"
	securityURL      = documentationURL + "security/"
)

// AccountSettings binds one Adjust app and its externally enabled products.
type AccountSettings struct {
	AppToken                  string `json:"app_token" yaml:"app_token"`
	SessionMeasurementEnabled bool   `json:"session_measurement_enabled,omitempty" yaml:"session_measurement_enabled,omitempty"`
	AdRevenuePackageEnabled   bool   `json:"ad_revenue_package_enabled,omitempty" yaml:"ad_revenue_package_enabled,omitempty"`
}

// Adapter implements socialhub.Adapter for the Adjust S2S API.
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
		return invalidArgument("init", "product must be s2s-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is required for every Adjust app")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" ||
			account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "OAuth client, secret, token store, app, and webhook settings are not used by this adapter")
		}
		if account.Approval.AccountType != "" {
			return invalidArgument("init", "approval.account_type is not used by the Adjust S2S API")
		}
		if !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval.scopes may contain only events, sessions, and ad_revenue")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validOpaque(typed.AppToken, 4096) {
			return invalidArgument("init", "account.settings.app_token is required")
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
	token, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: token, TokenType: "Bearer"}}
	api, err := transport.New(defaultBaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock))
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, appToken: typed.AppToken, api: api, clock: resolved.Clock,
		scopes:         append([]string(nil), account.Approval.Scopes...),
		sessionEnabled: typed.SessionMeasurementEnabled, adRevenueEnabled: typed.AdRevenuePackageEnabled,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Adjust S2S Security token")
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved Adjust S2S Security token is invalid")
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	base := copy.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = adjustStatusTransport{base: base}
	return &copy
}

// Adjust uses HTTP 202 for missing or invalid S2S authentication. Normalize it
// before the shared transport's generic 2xx handling, while preserving the
// wire status for diagnostics.
type adjustStatusTransport struct{ base http.RoundTripper }

func (transport adjustStatusTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request)
	if err == nil && response != nil {
		if response.Header != nil {
			response.Header.Del(originalStatusHeader)
		}
		if response.StatusCode == http.StatusAccepted {
			if response.Header == nil {
				response.Header = make(http.Header)
			}
			response.Header.Set(originalStatusHeader, "202")
			response.StatusCode = http.StatusUnauthorized
			response.Status = "401 Unauthorized"
		}
	}
	return response, err
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
