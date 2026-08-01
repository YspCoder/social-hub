// Package line implements the official LINE Messaging API.
package line

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
	adapterName        = "line/messaging-api"
	productName        = "messaging-api"
	apiVersion         = "v2"
	defaultBaseURL     = "https://api.line.me"
	defaultDataBaseURL = "https://api-data.line.me"
	docURL             = "https://developers.line.biz/en/reference/messaging-api/"
)

// Settings controls LINE API origins. Overrides are primarily useful for
// deterministic contract tests and approved gateways.
type Settings struct {
	BaseURL      string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	DataBaseURL  string `json:"data_base_url,omitempty" yaml:"data_base_url,omitempty"`
	TokenBaseURL string `json:"token_base_url,omitempty" yaml:"token_base_url,omitempty"`
}

// AccountSettings optionally pins the bot user ID expected in webhook bodies.
type AccountSettings struct {
	BotUserID string `json:"bot_user_id,omitempty" yaml:"bot_user_id,omitempty"`
}

// Adapter implements socialhub.Adapter for LINE Messaging API channels.
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
	settings := Settings{BaseURL: defaultBaseURL, DataBaseURL: defaultDataBaseURL, TokenBaseURL: defaultBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.DataBaseURL, settings.TokenBaseURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "LINE endpoints must be absolute HTTP(S) URLs without credentials")
		}
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every LINE channel")
		}
		if strings.TrimSpace(account.ClientID) != "" && strings.TrimSpace(account.SecretRef) == "" {
			return invalidArgument("init", "secret_ref is required when client_id is configured")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.BotUserID != "" && !validLINEID(typed.BotUserID, 'U') {
			return invalidArgument("init", "account.settings.bot_user_id must be a LINE user ID")
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
	channelSecret := ""
	if account.SecretRef != "" {
		channelSecret, err = resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	apiTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "line", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	dataTransport, err := transport.New(settings.DataBaseURL, resolved.HTTPClient, tokens, "line", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, botUserID: typed.BotUserID, api: apiTransport, data: dataTransport,
		httpClient: resolved.HTTPClient, channelSecret: channelSecret, clock: resolved.Clock,
	}, nil
}

// Tokens returns a helper for issuing, verifying, and revoking channel access
// tokens with a configured channel ID and channel secret.
func (a *Adapter) Tokens(ctx context.Context, accountID socialhub.AccountID) (*TokenClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("tokens", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	settings, options := a.settings, a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformError("tokens", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if strings.TrimSpace(account.ClientID) == "" || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("tokens", "client_id and secret_ref are required for token management")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "tokens")
	if err != nil {
		return nil, err
	}
	return &TokenClient{
		ChannelID: account.ClientID, ChannelSecret: secret, BaseURL: settings.TokenBaseURL,
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
