// Package soundcloud implements the SoundCloud Public API.
package soundcloud

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName     = "soundcloud/public-api-v1"
	productName     = "soundcloud-public-api"
	apiVersion      = "1.0.0"
	defaultBaseURL  = "https://api.soundcloud.com"
	defaultAuthURL  = "https://secure.soundcloud.com/authorize"
	defaultTokenURL = "https://secure.soundcloud.com/oauth/token"
	docURL          = "https://developers.soundcloud.com/docs/api/reference"
)

// Settings controls SoundCloud API and OAuth endpoints.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings optionally records the SoundCloud user represented by an account.
type AccountSettings struct {
	UserURN string `json:"user_urn,omitempty" yaml:"user_urn,omitempty"`
}

// Adapter implements socialhub.Adapter for the SoundCloud Public API.
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
		Name: adapterName, Product: productName, APIVersion: apiVersion,
		DocURL: docURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
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
	settings := Settings{BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all SoundCloud endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.UserURN != "" && !validURN(typed.UserURN, "users") {
			return invalidArgument("init", "account.settings.user_urn must be a SoundCloud user URN")
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
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "OAuth"}}
	api, err := transport.NewWithAuthenticator(settings.BaseURL, resolved.HTTPClient, tokens, "soundcloud", productName,
		transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
			request.Header.Set("Authorization", "OAuth "+token.AccessToken)
			return nil
		}), decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	client := &Client{
		accountID: accountID, userURN: typed.UserURN, accountType: account.Approval.AccountType,
		api: api, apiBaseURL: baseURL, clock: resolved.Clock,
	}
	client.tracks = &TrackUploadService{client: client}
	return client, nil
}

// OAuth returns a SoundCloud OAuth 2.1 helper for the configured application.
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
	if strings.TrimSpace(account.ClientID) == "" || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || strings.TrimSpace(secret) == "" {
		return nil, platformError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: settings.TokenURL, HTTPClient: options.HTTPClient, Clock: options.Clock,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validURN(value, resource string) bool {
	prefix := "soundcloud:" + resource + ":"
	if strings.TrimSpace(value) != value {
		return false
	}
	id := strings.TrimPrefix(value, prefix)
	if id == value || id == "" {
		return false
	}
	for _, character := range id {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

var _ socialhub.Adapter = (*Adapter)(nil)
