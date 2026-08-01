// Package instagram implements Instagram API with Instagram Login.
package instagram

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
	adapterName         = "instagram/login-v26"
	graphVersion        = "v26.0"
	defaultAPIURL       = "https://graph.instagram.com/v26.0"
	defaultAuthURL      = "https://www.instagram.com/oauth/authorize"
	defaultTokenURL     = "https://api.instagram.com/oauth/access_token"
	defaultLongTokenURL = "https://graph.instagram.com/access_token"
	defaultRefreshURL   = "https://graph.instagram.com/refresh_access_token"
	docURL              = "https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/"
)

// CapabilityContainerPublish identifies Instagram's media-container workflow.
const CapabilityContainerPublish socialhub.Capability = "container_publish"

// Settings controls Instagram Login and Graph endpoints.
type Settings struct {
	BaseURL      string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL      string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL     string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	LongTokenURL string `json:"long_token_url,omitempty" yaml:"long_token_url,omitempty"`
	RefreshURL   string `json:"refresh_url,omitempty" yaml:"refresh_url,omitempty"`
}

// AccountSettings identifies one Instagram professional account.
type AccountSettings struct {
	UserID string `json:"user_id" yaml:"user_id"`
}

// Adapter implements socialhub.Adapter for Instagram Login.
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
		Name: adapterName, Product: "instagram-login", APIVersion: graphVersion, DocURL: docURL,
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
	settings := Settings{
		BaseURL: defaultAPIURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL,
		LongTokenURL: defaultLongTokenURL, RefreshURL: defaultRefreshURL,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL, settings.LongTokenURL, settings.RefreshURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all Instagram endpoints must be absolute HTTP(S) URLs")
		}
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef == "" {
			return invalidArgument("init", "access_token_ref is required for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil || strings.TrimSpace(typed.UserID) == "" {
			return invalidArgument("init", "account.settings.user_id is required")
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
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	webhookSecret, err := resolveOptional(ctx, resolved.Secrets, account.Webhook.SecretRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	webhookToken, err := resolveOptional(ctx, resolved.Secrets, account.Webhook.TokenRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	httpTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "instagram", "instagram-login", decodeHTTPError)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, userID: typed.UserID, transport: httpTransport,
		webhookSecret: webhookSecret, webhookToken: webhookToken, scopes: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
	}
	client.containers = &ContainerService{client: client}
	return client, nil
}

// OAuth returns an Instagram Login OAuth helper for one configured app.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, wrapError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	settings := a.settings
	options := a.options
	a.mu.RUnlock()
	if !found {
		return nil, wrapError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if account.ClientID == "" || account.SecretRef == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil {
		return nil, wrapError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL,
		LongTokenURL: settings.LongTokenURL, RefreshURL: settings.RefreshURL, HTTPClient: options.HTTPClient,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = false
	a.closed = true
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
