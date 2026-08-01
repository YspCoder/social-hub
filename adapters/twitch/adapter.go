// Package twitch implements the official Twitch Helix API.
package twitch

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
	adapterName        = "twitch/helix"
	productName        = "helix"
	apiVersion         = "helix"
	defaultBaseURL     = "https://api.twitch.tv/helix"
	defaultAuthURL     = "https://id.twitch.tv/oauth2/authorize"
	defaultTokenURL    = "https://id.twitch.tv/oauth2/token"
	defaultValidateURL = "https://id.twitch.tv/oauth2/validate"
	defaultRevokeURL   = "https://id.twitch.tv/oauth2/revoke"
	docURL             = "https://dev.twitch.tv/docs/"
)

// Settings controls Helix and OAuth endpoints.
type Settings struct {
	BaseURL     string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL     string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL    string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	ValidateURL string `json:"validate_url,omitempty" yaml:"validate_url,omitempty"`
	RevokeURL   string `json:"revoke_url,omitempty" yaml:"revoke_url,omitempty"`
}

// AccountSettings configures one Twitch user and optional EventSub secrets.
type AccountSettings struct {
	UserID            string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
	AppAccessTokenRef string `json:"app_access_token_ref,omitempty" yaml:"app_access_token_ref,omitempty"`
	EventSubSecretRef string `json:"eventsub_secret_ref,omitempty" yaml:"eventsub_secret_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Twitch Helix.
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
		DocURL: docURL + "api/", VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
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
	settings := Settings{
		BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL,
		ValidateURL: defaultValidateURL, RevokeURL: defaultRevokeURL,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL, settings.ValidateURL, settings.RevokeURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all Twitch endpoints must be absolute HTTP(S) URLs without credentials")
		}
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.ClientID) == "" || account.AccessTokenRef == "" {
			return invalidArgument("init", "client_id and access_token_ref are required for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return invalidArgument("init", "account settings are invalid")
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
	accessToken, err := resolveRequiredSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	authenticator := helixAuthenticator{clientID: account.ClientID}
	primary, err := newHelixTransport(settings.BaseURL, resolved.HTTPClient, authenticator, accessToken)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	appTransport := primary
	if typed.AppAccessTokenRef != "" {
		appToken, resolveErr := resolveRequiredSecret(ctx, resolved.Secrets, typed.AppAccessTokenRef, "client_app_token")
		if resolveErr != nil {
			return nil, resolveErr
		}
		appTransport, err = newHelixTransport(settings.BaseURL, resolved.HTTPClient, authenticator, appToken)
		if err != nil {
			return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	webhookSecret := ""
	if typed.EventSubSecretRef != "" {
		webhookSecret, err = resolveRequiredSecret(ctx, resolved.Secrets, typed.EventSubSecretRef, "client_eventsub_secret")
		if err != nil {
			return nil, err
		}
		if !validEventSubSecret(webhookSecret) {
			return nil, invalidArgument("client_eventsub_secret", "EventSub secret must be 10-100 printable ASCII characters")
		}
	}
	return &Client{
		accountID: accountID, userID: strings.TrimSpace(typed.UserID), clientID: account.ClientID,
		transport: primary, appTransport: appTransport, scopes: append([]string(nil), account.Approval.Scopes...),
		webhookSecret: webhookSecret, clock: resolved.Clock,
	}, nil
}

func newHelixTransport(baseURL string, httpClient *http.Client, authenticator helixAuthenticator, token string) (*transport.Client, error) {
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: token, TokenType: "Bearer"}}
	return transport.NewWithAuthenticator(baseURL, httpClient, tokens, "twitch", productName, authenticator, decodeHTTPError)
}

func resolveRequiredSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

// OAuth returns a Twitch OAuth2 helper for one configured app.
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
	secret, err := resolveRequiredSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL,
		ValidateURL: settings.ValidateURL, RevokeURL: settings.RevokeURL, HTTPClient: options.HTTPClient, Clock: options.Clock,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

type helixAuthenticator struct{ clientID string }

func (a helixAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if strings.TrimSpace(a.clientID) == "" || strings.TrimSpace(token.AccessToken) == "" {
		return socialhub.ErrUnauthenticated
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header.Set("Client-Id", a.clientID)
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}
