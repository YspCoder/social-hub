// Package kakao implements Kakao Login and Kakao Talk REST APIs.
package kakao

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
	adapterName     = "kakao/login-talk-rest"
	productName     = "login-talk"
	apiVersion      = "REST v1/v2"
	defaultBaseURL  = "https://kapi.kakao.com"
	defaultAuthURL  = "https://kauth.kakao.com/oauth/authorize"
	defaultTokenURL = "https://kauth.kakao.com/oauth/token"
	docURL          = "https://developers.kakao.com/docs/en/kakaologin/rest-api"
	approvalURL     = "https://developers.kakao.com/console/app"
)

// Settings controls Kakao API and OAuth endpoints. Overrides are intended for
// deterministic tests and approved gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings identifies the Kakao Login user and optional common-message
// defaults.
type AccountSettings struct {
	UserID                string `json:"user_id" yaml:"user_id"`
	DefaultLinkURL        string `json:"default_link_url,omitempty" yaml:"default_link_url,omitempty"`
	FriendMessageApproved bool   `json:"friend_message_approved,omitempty" yaml:"friend_message_approved,omitempty"`
}

// Adapter implements socialhub.Adapter for Kakao Login and Kakao Talk.
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
		Name: adapterName, Product: productName, APIVersion: apiVersion, DocURL: docURL,
		VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
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
	if !validEndpoint(settings.BaseURL, false) || !validEndpoint(settings.AuthURL, true) || !validEndpoint(settings.TokenURL, true) {
		return invalidArgument("init", "Kakao endpoints must be absolute HTTP(S) URLs without credentials, queries, or fragments")
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every account")
		}
		if strings.TrimSpace(account.SecretRef) != "" && strings.TrimSpace(account.ClientID) == "" {
			return invalidArgument("init", "client_id is required when secret_ref configures the OAuth client secret")
		}
		if account.Webhook.SecretRef != "" || account.Webhook.TokenRef != "" || account.Webhook.AESKeyRef != "" {
			return invalidArgument("init", "Kakao webhooks are not implemented by this adapter")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validServiceUserID(typed.UserID) {
			return invalidArgument("init", "account.settings.user_id must be a positive decimal Kakao service user ID")
		}
		if typed.DefaultLinkURL != "" && !validHTTPURL(typed.DefaultLinkURL) {
			return invalidArgument("init", "account.settings.default_link_url must be an absolute HTTP(S) URL without credentials or fragments")
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
	api, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "kakao", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, userID: typed.UserID, defaultLinkURL: typed.DefaultLinkURL,
		friendMessageApproved: typed.FriendMessageApproved, api: api,
	}, nil
}

// OAuth returns a Kakao OAuth 2.0 authorization-code and refresh helper.
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
	if strings.TrimSpace(account.ClientID) == "" {
		return nil, invalidArgument("oauth", "client_id is required")
	}
	secret := ""
	if strings.TrimSpace(account.SecretRef) != "" {
		var err error
		secret, err = options.Secrets.Resolve(ctx, account.SecretRef)
		if err != nil || strings.TrimSpace(secret) == "" {
			return nil, platformError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
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

func validEndpoint(value string, allowPath bool) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == "" && parsed.RawQuery == "" && (allowPath || parsed.Path == "" || parsed.Path == "/")
}

var _ socialhub.Adapter = (*Adapter)(nil)
