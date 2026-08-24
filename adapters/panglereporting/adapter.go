// Package panglereporting implements Pangle Publisher Reporting API 2.0.
package panglereporting

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName            = "pangle/publisher-reporting-api-v2"
	platformName           = "pangle"
	productName            = "publisher-reporting-api"
	apiVersion             = "2.0"
	defaultBaseURL         = "https://open-api.pangleglobal.com"
	documentationURL       = "https://www.pangleglobal.com/integration/reporting-api-v2"
	defaultMaxResponseSize = int64(128 << 20)
	maximumResponseSize    = int64(512 << 20)
)

// Settings controls the maximum decoded JSON size.
type Settings struct {
	MaxResponseBytes int64 `json:"max_response_bytes,omitempty" yaml:"max_response_bytes,omitempty"`
}

// AccountSettings binds one social-hub account to one Pangle master account
// and role. The role's Security Key is resolved separately from secret_ref.
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
		return invalidArgument("init", "product must be publisher-reporting-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{MaxResponseBytes: defaultMaxResponseSize}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.MaxResponseBytes < 1<<20 || settings.MaxResponseBytes > maximumResponseSize {
		return invalidArgument("init", "settings.max_response_bytes must be between 1 MiB and 512 MiB")
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
			return invalidArgument("init", "webhook settings are not supported by Reporting API 2.0")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth approval fields are not used by Reporting API 2.0")
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
	baseURL, _ := url.Parse(defaultBaseURL)
	return &Client{
		accountID: accountID, userID: typed.UserID, roleID: typed.RoleID, securityKey: securityKey,
		baseURL: baseURL, httpClient: cloneHTTPClient(resolved.HTTPClient), clock: resolved.Clock,
		maxResponseBytes: settings.MaxResponseBytes,
	}, nil
}

func resolveSecurityKey(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 4096) {
		return "", authenticationError("client", err)
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
