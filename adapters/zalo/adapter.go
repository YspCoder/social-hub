// Package zalo implements the Zalo Official Account OpenAPI.
package zalo

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
	adapterName         = "zalo/official-account"
	productName         = "oa-openapi"
	apiVersion          = "v3.0/v2.0"
	defaultBaseURL      = "https://openapi.zalo.me"
	defaultOAuthBaseURL = "https://oauth.zaloapp.com"
	docURL              = "https://developers.zalo.me/docs/official-account/bat-dau/kham-pha"
)

// Settings controls Zalo API origins. Overrides are intended for deterministic
// contract tests and approved gateways.
type Settings struct {
	BaseURL      string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	OAuthBaseURL string `json:"oauth_base_url,omitempty" yaml:"oauth_base_url,omitempty"`
}

// AccountSettings identifies the OA represented by an access token. OAID is
// optional, but when configured it is used to reject misrouted webhooks.
type AccountSettings struct {
	OAID string `json:"oa_id,omitempty" yaml:"oa_id,omitempty"`
}

// Adapter implements socialhub.Adapter for one or more Zalo Official Accounts.
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
	settings := Settings{BaseURL: defaultBaseURL, OAuthBaseURL: defaultOAuthBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.OAuthBaseURL) {
		return invalidArgument("init", "Zalo endpoints must be absolute HTTP(S) URLs without credentials")
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every Zalo OA")
		}
		if account.AppID != "" && !validNumericID(account.AppID) {
			return invalidArgument("init", "app_id must be a decimal Zalo application ID")
		}
		if account.SecretRef != "" && account.AppID == "" {
			return invalidArgument("init", "app_id is required when secret_ref configures OAuth")
		}
		if account.Webhook.SecretRef != "" && account.AppID == "" {
			return invalidArgument("init", "app_id is required when webhook.secret_ref is configured")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.OAID != "" && !validNumericID(typed.OAID) {
			return invalidArgument("init", "account.settings.oa_id must be a decimal Zalo OA ID")
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	webhookSecret := ""
	if account.Webhook.SecretRef != "" {
		webhookSecret, err = resolveSecret(ctx, resolved.Secrets, account.Webhook.SecretRef, "client")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if strings.TrimSpace(token.AccessToken) == "" {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("access_token", token.AccessToken)
		return nil
	})
	api, err := transport.NewWithAuthenticator(settings.BaseURL, resolved.HTTPClient, tokens, "zalo", productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, appID: account.AppID, oaID: typed.OAID,
		webhookSecret: webhookSecret,
	}, nil
}

// OAuth returns a helper for OAuth v4 authorization-code exchange and
// rotating refresh-token calls. The authorization URL itself is configured in
// the Zalo developer console and is therefore not synthesized by this SDK.
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
	if !validNumericID(account.AppID) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "app_id and secret_ref are required for OAuth")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		AppID: account.AppID, AppSecret: secret, BaseURL: settings.OAuthBaseURL,
		HTTPClient: options.HTTPClient, Clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
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

func validNumericID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 32 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validOpaqueID(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
