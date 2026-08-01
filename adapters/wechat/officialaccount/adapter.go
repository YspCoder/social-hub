// Package officialaccount implements the WeChat Official Account API adapter.
package officialaccount

import (
	"context"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName   = "wechat/official-account"
	defaultAPIURL = "https://api.weixin.qq.com"
	docURL        = "https://developers.weixin.qq.com/doc/offiaccount/Getting_Started/Overview.html"
)

// Settings controls the Official Account API origin.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// Adapter implements socialhub.Adapter for WeChat Official Accounts.
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
		Product:    "official-account",
		APIVersion: "continuous",
		DocURL:     docURL,
		VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != "" && config.Adapter != adapterName {
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
		if account.AppID == "" {
			return invalidArgument("init", "app_id is required for every account")
		}
		if account.AccessTokenRef == "" && account.SecretRef == "" {
			return invalidArgument("init", "secret_ref or access_token_ref is required")
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

	var tokens socialhub.TokenSource
	if account.AccessTokenRef != "" {
		accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
		if err != nil {
			return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken}}
	} else {
		secret, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
		if err != nil {
			return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
		tokens = &appTokenSource{baseURL: settings.BaseURL, appID: account.AppID, secret: secret, httpClient: resolved.HTTPClient, clock: resolved.Clock}
	}
	httpTransport, err := transport.NewWithAuthenticator(settings.BaseURL, resolved.HTTPClient, tokens, "wechat", "official-account", transport.QueryAuthenticator("access_token"), decodeHTTPError)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	webhookToken, err := resolveOptional(ctx, resolved.Secrets, account.Webhook.TokenRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	aesKey, err := resolveOptional(ctx, resolved.Secrets, account.Webhook.AESKeyRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	client := &Client{
		accountID:    accountID,
		appID:        account.AppID,
		transport:    httpTransport,
		webhookToken: webhookToken,
		aesKey:       aesKey,
		clock:        resolved.Clock,
		uploads:      make(map[string]*uploadState),
		assets:       make(map[string]*materialAsset),
	}
	client.materials = &MaterialService{client: client}
	client.drafts = &DraftService{client: client}
	return client, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = false
	a.closed = true
	return nil
}

func resolveOptional(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	if reference == "" {
		return "", nil
	}
	return resolver.Resolve(ctx, reference)
}
