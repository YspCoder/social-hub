// Package vk implements VK API 5.199.
package vk

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
	adapterName    = "vk/v5.199"
	productName    = "api"
	apiVersion     = "5.199"
	defaultBaseURL = "https://api.vk.ru/method"
	docURL         = "https://dev.vk.ru/ru/reference"
)

// TokenKind identifies the VK access-token actor and determines which method
// families can be called safely.
type TokenKind string

const (
	TokenUser      TokenKind = "user"
	TokenCommunity TokenKind = "community"
	TokenService   TokenKind = "service"
)

// Settings controls the VK API origin. BaseURL overrides are primarily useful
// for deterministic contract tests and approved gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings selects the default wall and declares the configured token
// type. Community walls use a negative owner ID.
type AccountSettings struct {
	OwnerID   int64     `json:"owner_id" yaml:"owner_id"`
	TokenKind TokenKind `json:"token_kind" yaml:"token_kind"`
}

// Adapter implements socialhub.Adapter for VK API accounts.
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
	settings := Settings{BaseURL: defaultBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an absolute HTTP(S) URL without credentials")
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every VK account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.OwnerID == 0 {
			return invalidArgument("init", "account.settings.owner_id must be a non-zero VK owner ID")
		}
		switch typed.TokenKind {
		case TokenUser, TokenCommunity, TokenService:
		default:
			return invalidArgument("init", "account.settings.token_kind must be user, community, or service")
		}
		if typed.TokenKind == TokenCommunity && typed.OwnerID >= 0 {
			return invalidArgument("init", "a community token requires a negative community owner_id")
		}
		if account.Webhook.SecretRef != "" && typed.OwnerID >= 0 {
			return invalidArgument("init", "Callback API verification requires a negative community owner_id")
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
	callbackSecret := ""
	if account.Webhook.SecretRef != "" {
		callbackSecret, err = resolveSecret(ctx, resolved.Secrets, account.Webhook.SecretRef, "client")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	apiTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "vk", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	groupID := int64(0)
	if typed.OwnerID < 0 {
		groupID = -typed.OwnerID
	}
	return &Client{
		accountID: accountID, ownerID: typed.OwnerID, groupID: groupID, tokenKind: typed.TokenKind,
		api: apiTransport, httpClient: resolved.HTTPClient, clock: resolved.Clock, callbackSecret: callbackSecret,
		allowHTTPUploads: baseURL.Scheme == "http",
		uploads:          make(map[string]*uploadState), media: make(map[string]socialhub.Media),
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
