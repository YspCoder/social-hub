// Package xiaohongshu implements the Xiaohongshu Share JS SDK server handoff.
package xiaohongshu

import (
	"context"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName   = "xiaohongshu/share-js"
	defaultAPIURL = "https://edith.xiaohongshu.com"
	docURL        = "https://agora.xiaohongshu.com/doc/js"
)

// CapabilityShare identifies the official client-side share handoff.
const CapabilityShare socialhub.Capability = "client_share"

// Settings controls the Share JS SDK credential endpoint.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

type accountSettings struct {
	Approved bool `json:"approved,omitempty" yaml:"approved,omitempty"`
}

// Adapter implements socialhub.Adapter for Xiaohongshu Share Open Platform.
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

func (a *Adapter) Name() string { return adapterName }

func (a *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name:       adapterName,
		Product:    "share-js",
		APIVersion: "1.0.1",
		DocURL:     docURL,
		VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultAPIURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.BaseURL == "" {
		return invalidArgument("init", "base_url is required")
	}
	for _, account := range config.Accounts {
		if account.ClientID == "" || account.SecretRef == "" {
			return invalidArgument("init", "client_id (appKey) and secret_ref are required for every account")
		}
		var typed accountSettings
		if len(account.Settings) > 0 {
			if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
				return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return wrapError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config = config
	a.options = resolved
	a.settings = settings
	a.ready = true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, wrapError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions := a.options
	settings := a.settings
	a.mu.RUnlock()
	if !found {
		return nil, wrapError("client", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	var typed accountSettings
	if len(account.Settings) > 0 {
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}

	combined := []socialhub.Option{
		socialhub.WithHTTPClient(baseOptions.HTTPClient),
		socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets),
		socialhub.WithClock(baseOptions.Clock),
	}
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	appSecret, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var staticToken socialhub.Token
	if account.AccessTokenRef != "" {
		value, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
		if err != nil {
			return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
		staticToken = socialhub.Token{AccessToken: value, TokenType: "XHSShare"}
	}
	client := &Client{accountID: accountID, appKey: account.ClientID, appSecret: appSecret, baseURL: settings.BaseURL, httpClient: resolved.HTTPClient, clock: resolved.Clock, tokenStore: resolved.TokenStore, token: staticToken, approved: typed.Approved}
	client.shares = &ShareService{client: client}
	return client, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = false
	a.closed = true
	return nil
}
