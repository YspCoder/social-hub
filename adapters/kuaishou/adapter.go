// Package kuaishou implements the Kuaishou Open Platform adapter.
package kuaishou

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName   = "kuaishou/openapi"
	defaultAPIURL = "https://open.kuaishou.com"
	docURL        = "https://open.kuaishou.com/platform/openApi"
)

// Settings controls Kuaishou API, OAuth, and dynamic upload endpoints.
type Settings struct {
	BaseURL            string   `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL            string   `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	OAuthBaseURL       string   `json:"oauth_base_url,omitempty" yaml:"oauth_base_url,omitempty"`
	UploadScheme       string   `json:"upload_scheme,omitempty" yaml:"upload_scheme,omitempty"`
	AllowedUploadHosts []string `json:"allowed_upload_hosts,omitempty" yaml:"allowed_upload_hosts,omitempty"`
}

type accountSettings struct {
	OpenID string `json:"open_id,omitempty" yaml:"open_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Kuaishou Open Platform.
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
		Product:    "openapi",
		APIVersion: "continuous",
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
	settings := Settings{
		BaseURL:      defaultAPIURL,
		AuthURL:      defaultAPIURL + "/oauth2/authorize",
		OAuthBaseURL: defaultAPIURL,
		UploadScheme: "https",
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.BaseURL == "" || settings.AuthURL == "" || settings.OAuthBaseURL == "" {
		return invalidArgument("init", "base_url, auth_url, and oauth_base_url are required")
	}
	if settings.UploadScheme != "https" && settings.UploadScheme != "http" {
		return invalidArgument("init", "upload_scheme must be https or http")
	}
	if settings.UploadScheme == "http" && len(settings.AllowedUploadHosts) == 0 {
		return invalidArgument("init", "HTTP upload endpoints require an explicit allowed_upload_hosts list")
	}
	for _, host := range settings.AllowedUploadHosts {
		if strings.TrimSpace(host) == "" || strings.ContainsAny(host, "/:@") {
			return invalidArgument("init", "allowed_upload_hosts entries must be hostnames without ports or schemes")
		}
		if settings.UploadScheme == "http" && host != "localhost" {
			address := net.ParseIP(host)
			if address == nil || !address.IsLoopback() {
				return invalidArgument("init", "HTTP upload endpoints are restricted to explicitly configured loopback hosts")
			}
		}
	}
	for _, account := range config.Accounts {
		if account.ClientID == "" {
			return invalidArgument("init", "client_id is required for every account")
		}
		if account.AccessTokenRef == "" && account.SecretRef == "" {
			return invalidArgument("init", "access_token_ref or secret_ref is required for every account")
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
	if account.AccessTokenRef == "" {
		return nil, &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "kuaishou", Product: "openapi", Op: "client", PlatformMessage: "a user access_token_ref is required for user APIs"}
	}
	var typed accountSettings
	if len(account.Settings) > 0 {
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if typed.OpenID == "" {
		return nil, invalidArgument("client", "account.settings.open_id is required for user APIs")
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
	httpTransport, err := transport.NewWithAuthenticator(
		settings.BaseURL,
		resolved.HTTPClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken}},
		"kuaishou",
		"openapi",
		transport.QueryAuthenticator("access_token"),
		decodeHTTPError,
	)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID:          accountID,
		appID:              account.ClientID,
		openID:             typed.OpenID,
		transport:          httpTransport,
		httpClient:         resolved.HTTPClient,
		clock:              resolved.Clock,
		uploadScheme:       settings.UploadScheme,
		allowedUploadHosts: append([]string(nil), settings.AllowedUploadHosts...),
		scopes:             append([]string(nil), account.Approval.Scopes...),
		uploads:            make(map[string]*uploadState),
		statuses:           make(map[string]socialhub.PublishStatus),
	}
	client.videos = &VideoService{client: client}
	return client, nil
}

// OAuth returns the user authorization-code helper for an account.
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
	return &OAuthClient{AppID: account.ClientID, AppSecret: secret, AuthURL: settings.AuthURL, TokenBaseURL: settings.OAuthBaseURL, HTTPClient: options.HTTPClient, Clock: options.Clock}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = false
	a.closed = true
	return nil
}
