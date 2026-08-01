// Package youtube implements YouTube Data API v3.
package youtube

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
	adapterName      = "youtube/data-v3"
	apiVersion       = "v3"
	defaultAPIURL    = "https://www.googleapis.com/youtube/v3"
	defaultUploadURL = "https://www.googleapis.com/upload/youtube/v3"
	defaultAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL  = "https://oauth2.googleapis.com/token"
	docURL           = "https://developers.google.com/youtube/v3/"
)

// CapabilityVideoUpload identifies YouTube's metadata plus resumable upload flow.
const CapabilityVideoUpload socialhub.Capability = "video_upload"

// Settings controls YouTube Data API and OAuth endpoints.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	UploadURL string `json:"upload_url,omitempty" yaml:"upload_url,omitempty"`
	AuthURL   string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL  string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings identifies one authorized YouTube channel.
type AccountSettings struct {
	ChannelID string `json:"channel_id" yaml:"channel_id"`
}

// Adapter implements socialhub.Adapter for YouTube Data API v3.
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
		Name: adapterName, Product: "youtube-data", APIVersion: apiVersion,
		DocURL: docURL + "docs", VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
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
	settings := Settings{BaseURL: defaultAPIURL, UploadURL: defaultUploadURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.UploadURL, settings.AuthURL, settings.TokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all YouTube endpoints must be absolute HTTP(S) URLs")
		}
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef == "" {
			return invalidArgument("init", "access_token_ref is required for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil || strings.TrimSpace(typed.ChannelID) == "" {
			return invalidArgument("init", "account.settings.channel_id is required")
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
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	apiTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "youtube", "youtube-data", decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	uploadTransport, err := transport.New(settings.UploadURL, resolved.HTTPClient, tokens, "youtube", "youtube-data", decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, channelID: typed.ChannelID, transport: apiTransport, uploadTransport: uploadTransport,
		httpClient: resolved.HTTPClient, accessToken: accessToken, scopes: append([]string(nil), account.Approval.Scopes...),
		clock: resolved.Clock, uploads: make(map[string]*videoUpload),
	}
	client.videos = &VideoUploadService{client: client}
	return client, nil
}

// OAuth returns a Google OAuth helper for the configured YouTube app.
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
	return &OAuthClient{ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL, HTTPClient: options.HTTPClient}, nil
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
