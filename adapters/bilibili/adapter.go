// Package bilibili implements the Bilibili Open Platform adapter.
package bilibili

import (
	"context"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName         = "bilibili/open-platform"
	defaultAPIURL       = "https://member.bilibili.com"
	defaultUploadURL    = "https://openupos.bilivideo.com"
	defaultOAuthURL     = "https://api.bilibili.com"
	defaultAuthorizeURL = "https://account.bilibili.com/pc/account-pc/auth/oauth"
	docURL              = "https://open.bilibili.com/doc"
)

// Settings controls Bilibili API, OAuth, and upload endpoints.
type Settings struct {
	BaseURL       string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	UploadBaseURL string `json:"upload_base_url,omitempty" yaml:"upload_base_url,omitempty"`
	AuthURL       string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	OAuthBaseURL  string `json:"oauth_base_url,omitempty" yaml:"oauth_base_url,omitempty"`
}

type accountSettings struct {
	OpenID           string   `json:"open_id,omitempty" yaml:"open_id,omitempty"`
	DefaultTID       int      `json:"default_tid,omitempty" yaml:"default_tid,omitempty"`
	DefaultTags      []string `json:"default_tags,omitempty" yaml:"default_tags,omitempty"`
	DefaultCopyright int      `json:"default_copyright,omitempty" yaml:"default_copyright,omitempty"`
	DefaultSource    string   `json:"default_source,omitempty" yaml:"default_source,omitempty"`
	NoReprint        bool     `json:"no_reprint,omitempty" yaml:"no_reprint,omitempty"`
}

// Adapter implements socialhub.Adapter for Bilibili Open Platform.
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
		Product:    "open-platform",
		APIVersion: "continuous-v2-signature",
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
	settings := Settings{BaseURL: defaultAPIURL, UploadBaseURL: defaultUploadURL, AuthURL: defaultAuthorizeURL, OAuthBaseURL: defaultOAuthURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.BaseURL == "" || settings.UploadBaseURL == "" || settings.AuthURL == "" || settings.OAuthBaseURL == "" {
		return invalidArgument("init", "base_url, upload_base_url, auth_url, and oauth_base_url are required")
	}
	for _, account := range config.Accounts {
		if account.ClientID == "" || account.SecretRef == "" {
			return invalidArgument("init", "client_id and secret_ref are required for every account")
		}
		var typed accountSettings
		if len(account.Settings) > 0 {
			if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
				return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
		}
		if typed.DefaultCopyright != 0 && typed.DefaultCopyright != 1 && typed.DefaultCopyright != 2 {
			return invalidArgument("init", "default_copyright must be 1 (original) or 2 (repost)")
		}
		if typed.DefaultCopyright == 2 && typed.DefaultSource == "" {
			return invalidArgument("init", "default_source is required when default_copyright is 2")
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
	if account.AccessTokenRef == "" {
		return nil, &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "bilibili", Product: "open-platform", Op: "client", PlatformMessage: "a user access_token_ref is required for Open Platform APIs"}
	}
	var typed accountSettings
	if len(account.Settings) > 0 {
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if typed.OpenID == "" {
		return nil, invalidArgument("client", "account.settings.open_id is required")
	}
	if typed.DefaultCopyright == 0 {
		typed.DefaultCopyright = 1
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
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	appSecret, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	token := socialhub.Token{AccessToken: accessToken, TokenType: "BilibiliUser"}
	signer := &requestSigner{clientID: account.ClientID, appSecret: appSecret, clock: resolved.Clock}
	httpTransport, err := transport.NewWithAuthenticator(settings.BaseURL, resolved.HTTPClient, socialhub.StaticTokenSource{Value: token}, "bilibili", "open-platform", signer, decodeHTTPError)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID:     accountID,
		openID:        typed.OpenID,
		baseURL:       settings.BaseURL,
		transport:     httpTransport,
		httpClient:    resolved.HTTPClient,
		uploadBaseURL: settings.UploadBaseURL,
		token:         token,
		signer:        signer,
		clock:         resolved.Clock,
		scopes:        append([]string(nil), account.Approval.Scopes...),
		defaults:      typed,
		uploads:       make(map[string]*uploadState),
	}
	client.submissions = &SubmissionService{client: client}
	client.videos = &VideoService{client: client}
	return client, nil
}

// OAuth returns the authorization-code helper for an account.
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
	secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil {
		return nil, wrapError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return &OAuthClient{ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenBaseURL: settings.OAuthBaseURL, HTTPClient: options.HTTPClient}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = false
	a.closed = true
	return nil
}
