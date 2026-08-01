// Package tiktok implements TikTok for Developers API v2.
package tiktok

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName     = "tiktok/v2"
	apiVersion      = "v2"
	defaultAPIURL   = "https://open.tiktokapis.com"
	defaultAuthURL  = "https://www.tiktok.com/v2/auth/authorize/"
	defaultTokenURL = "https://open.tiktokapis.com/v2/oauth/token/"
	docURL          = "https://developers.tiktok.com/doc/"
)

// CapabilityContentPosting identifies TikTok's asynchronous posting workflow.
const CapabilityContentPosting socialhub.Capability = "content_posting"

// Settings controls TikTok API and OAuth endpoints.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings identifies one TikTok-authorized user.
type AccountSettings struct {
	OpenID string `json:"open_id" yaml:"open_id"`
}

// Adapter implements socialhub.Adapter for TikTok API v2.
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
		Name: adapterName, Product: "tiktok-for-developers", APIVersion: apiVersion,
		DocURL: docURL, VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
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
	settings := Settings{BaseURL: defaultAPIURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all TikTok endpoints must be absolute HTTP(S) URLs")
		}
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef == "" {
			return invalidArgument("init", "access_token_ref is required for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil || strings.TrimSpace(typed.OpenID) == "" {
			return invalidArgument("init", "account.settings.open_id is required")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.settings, a.ready = config, resolved, settings, true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions, settings := a.options, a.settings
	a.mu.RUnlock()
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
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || strings.TrimSpace(accessToken) == "" {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	webhookRef := account.Webhook.SecretRef
	if webhookRef == "" {
		webhookRef = account.SecretRef
	}
	webhookSecret, err := resolveOptional(ctx, resolved.Secrets, webhookRef)
	if err != nil {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	httpTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "tiktok", "tiktok-for-developers", decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, openID: typed.OpenID, clientKey: account.ClientID,
		transport: httpTransport, httpClient: resolved.HTTPClient, scopes: append([]string(nil), account.Approval.Scopes...),
		webhookSecret: webhookSecret, clock: resolved.Clock, uploads: make(map[string]*videoUpload),
	}
	client.content = &ContentService{client: client}
	return client, nil
}

// OAuth returns a TikTok Login Kit OAuth helper for one configured app.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	settings, options := a.settings, a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if account.ClientID == "" || account.SecretRef == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || secret == "" {
		return nil, platformError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return &OAuthClient{ClientKey: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL, HTTPClient: options.HTTPClient}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func resolveOptional(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	if reference == "" {
		return "", nil
	}
	return resolver.Resolve(ctx, reference)
}
