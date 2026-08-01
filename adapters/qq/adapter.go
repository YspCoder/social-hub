// Package qq implements the official QQ Bot OpenAPI.
package qq

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName     = "qq/bot-api"
	productName     = "bot-api"
	apiVersion      = "v2"
	defaultBaseURL  = "https://api.bot.qq.com"
	defaultTokenURL = "https://api.bot.qq.com/app/getAppAccessToken"
	docURL          = "https://bot.q.qq.com/wiki/develop/api-v2/"
)

// Settings controls the QQ OpenAPI and token endpoints. Overrides are intended
// for deterministic tests and approved gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

type accountSettings struct{}

// Adapter implements socialhub.Adapter for QQ bots.
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
		VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
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
	settings := Settings{BaseURL: defaultBaseURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL, false) || !validEndpoint(settings.TokenURL, true) {
		return invalidArgument("init", "base_url and token_url must be absolute HTTP(S) URLs without credentials or fragments")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AppID, 128) {
			return invalidArgument("init", "app_id is required for every QQ bot account")
		}
		if strings.TrimSpace(account.AccessTokenRef) == "" && strings.TrimSpace(account.SecretRef) == "" {
			return invalidArgument("init", "secret_ref or access_token_ref is required for every account")
		}
		if account.Webhook.TokenRef != "" || account.Webhook.AESKeyRef != "" {
			return invalidArgument("init", "QQ webhook verification uses AppSecret via secret_ref or webhook.secret_ref")
		}
		var typed accountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
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
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}

	appSecret, err := resolveOptionalSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	var tokens socialhub.TokenSource
	var invalidator tokenInvalidator
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "QQBot"}}
	} else {
		source := &appTokenSource{
			tokenURL: settings.TokenURL, appID: account.AppID, secret: appSecret,
			httpClient: resolved.HTTPClient, clock: resolved.Clock, store: resolved.TokenStore,
			key: socialhub.TokenKey{Platform: "qq", Product: productName, Tenant: account.AppID, Account: string(accountID)},
		}
		tokens, invalidator = source, source
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "QQBot "+token.AccessToken)
		return nil
	})
	errorDecoder := func(status int, header http.Header, body []byte) error {
		decoded := decodeHTTPError(status, header, body)
		var platformErr *socialhub.Error
		if invalidator != nil && errors.As(decoded, &platformErr) && platformErr.Code == socialhub.CodeUnauthenticated {
			invalidator.Invalidate(context.Background())
		}
		return decoded
	}
	api, err := transport.NewWithAuthenticator(settings.BaseURL, resolved.HTTPClient, tokens, "qq", productName, authenticator, errorDecoder)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	webhookSecret := appSecret
	if account.Webhook.SecretRef != "" {
		webhookSecret, err = resolveSecret(ctx, resolved.Secrets, account.Webhook.SecretRef, "client")
		if err != nil {
			return nil, err
		}
	}
	if webhookSecret != "" && len(webhookSecret) > 512 {
		return nil, invalidArgument("client", "AppSecret exceeds 512 bytes")
	}
	return &Client{
		accountID: accountID, appID: account.AppID, api: api, clock: resolved.Clock,
		invalidator: invalidator, webhookSecret: webhookSecret,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func resolveOptionalSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	if strings.TrimSpace(reference) == "" {
		return "", nil
	}
	return resolveSecret(ctx, resolver, reference, operation)
}

func validEndpoint(value string, allowPath bool) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == "" && parsed.RawQuery == "" && (allowPath || parsed.Path == "" || parsed.Path == "/")
}

var _ socialhub.Adapter = (*Adapter)(nil)
