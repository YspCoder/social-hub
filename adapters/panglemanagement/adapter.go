// Package panglemanagement implements Pangle App and Ad Placement Management API.
package panglemanagement

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName       = "pangle/app-placement-management-api-v1.1.13"
	platformName      = "pangle"
	productName       = "app-placement-management-api"
	apiVersion        = "1.1.13"
	wireVersion       = "1.0"
	defaultBaseURL    = "https://open-api.pangleglobal.com"
	defaultSandboxURL = "http://open-api-sandbox.pangleglobal.com"
	documentationURL  = "https://www.pangleglobal.com/integration/management-api"
)

type Settings struct {
	Sandbox bool `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
}

type AccountSettings struct {
	UserID string `json:"user_id" yaml:"user_id"`
	RoleID string `json:"role_id" yaml:"role_id"`
}

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
		return invalidArgument("init", "product must be app-placement-management-api")
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
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validNumericID(typed.UserID) || !validNumericID(typed.RoleID) {
			return invalidArgument("init", "account.settings requires positive decimal user_id and role_id values")
		}
		if !validOpaque(account.SecretRef, 4_096) || account.ClientID != "" || account.AppID != "" ||
			account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "configure only secret_ref with the Pangle role Security Key")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by the Management API")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth approval fields are not used by the Management API")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	securityKey, err := resolveSecurityKey(ctx, resolved.Secrets, account.SecretRef)
	if err != nil {
		return nil, err
	}
	endpoint := defaultBaseURL
	if settings.Sandbox {
		endpoint = defaultSandboxURL
	}
	baseURL, _ := url.Parse(endpoint)
	return &Client{
		accountID: accountID, userID: ID(typed.UserID), roleID: ID(typed.RoleID), securityKey: securityKey,
		baseURL: baseURL, httpClient: cloneHTTPClient(resolved.HTTPClient), clock: resolved.Clock,
		sandbox: settings.Sandbox,
	}, nil
}

func resolveSecurityKey(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 4096) {
		return "", authenticationError("client", "Pangle Security Key resolution failed", err, reference, value)
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
