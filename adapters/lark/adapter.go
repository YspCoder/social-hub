// Package lark implements Feishu and Lark Open Platform adapters.
package lark

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
	adapterName          = "lark/openapi"
	productName          = "open-platform"
	apiVersion           = "IM v1 / Contact v3"
	defaultFeishuBaseURL = "https://open.feishu.cn"
	defaultLarkBaseURL   = "https://open.larksuite.com"
	docURL               = "https://open.feishu.cn/document/server-docs/"
)

// Region selects the China or international Open Platform origin.
type Region string

const (
	RegionFeishu Region = "feishu"
	RegionLark   Region = "lark"
)

// TokenKind identifies the actor represented by an access token.
type TokenKind string

const (
	TokenTenant TokenKind = "tenant"
	TokenUser   TokenKind = "user"
)

// UserIDType identifies a user in Contact and IM APIs.
type UserIDType string

const (
	UserIDOpenID  UserIDType = "open_id"
	UserIDUnionID UserIDType = "union_id"
	UserIDUserID  UserIDType = "user_id"
)

// Settings overrides the Open Platform origin. BaseURL is intended for
// deterministic tests and approved gateways; normal clients select an origin
// from each account's region.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings identifies one Feishu or Lark installation.
type AccountSettings struct {
	Region        Region     `json:"region" yaml:"region"`
	TokenKind     TokenKind  `json:"token_kind" yaml:"token_kind"`
	UserIDType    UserIDType `json:"user_id_type,omitempty" yaml:"user_id_type,omitempty"`
	ActorID       string     `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	TenantKey     string     `json:"tenant_key,omitempty" yaml:"tenant_key,omitempty"`
	DefaultChatID string     `json:"default_chat_id,omitempty" yaml:"default_chat_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Feishu and Lark installations.
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
	var settings Settings
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.BaseURL != "" && !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every Feishu/Lark account")
		}
		if account.Webhook.AESKeyRef != "" && account.Webhook.TokenRef == "" {
			return invalidArgument("init", "webhook.token_ref is required when webhook.aes_key_ref is configured")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.Region != RegionFeishu && typed.Region != RegionLark {
			return invalidArgument("init", "account.settings.region must be feishu or lark")
		}
		if typed.TokenKind != TokenTenant && typed.TokenKind != TokenUser {
			return invalidArgument("init", "account.settings.token_kind must be tenant or user")
		}
		if typed.UserIDType == "" {
			typed.UserIDType = UserIDOpenID
		}
		if !validUserIDType(typed.UserIDType) {
			return invalidArgument("init", "account.settings.user_id_type must be open_id, union_id, or user_id")
		}
		if typed.ActorID != "" && !validOpaqueID(typed.ActorID, 512) {
			return invalidArgument("init", "account.settings.actor_id must be a bounded opaque user or app ID")
		}
		if typed.TenantKey != "" && !validOpaqueID(typed.TenantKey, 256) {
			return invalidArgument("init", "account.settings.tenant_key must be a bounded opaque tenant key")
		}
		if typed.DefaultChatID != "" && !validChatID(typed.DefaultChatID) {
			return invalidArgument("init", "account.settings.default_chat_id must be a Lark chat ID")
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
	verificationToken, err := resolveOptionalSecret(ctx, resolved.Secrets, account.Webhook.TokenRef, "client")
	if err != nil {
		return nil, err
	}
	encryptKey, err := resolveOptionalSecret(ctx, resolved.Secrets, account.Webhook.AESKeyRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if typed.UserIDType == "" {
		typed.UserIDType = UserIDOpenID
	}
	baseURL := settings.BaseURL
	if baseURL == "" {
		baseURL = baseURLFor(typed.Region)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	api, err := transport.New(baseURL, resolved.HTTPClient, tokens, "lark", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, appID: strings.TrimSpace(account.AppID), tenantKey: typed.TenantKey,
		region: typed.Region, tokenKind: typed.TokenKind, userIDType: typed.UserIDType,
		actorID: typed.ActorID, defaultChatID: typed.DefaultChatID, api: api, clock: resolved.Clock,
		verificationToken: verificationToken, encryptKey: encryptKey,
		scopes:  append([]string(nil), account.Approval.Scopes...),
		uploads: make(map[string]*uploadState), media: make(map[string]socialhub.Media),
	}, nil
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

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func baseURLFor(region Region) string {
	if region == RegionLark {
		return defaultLarkBaseURL
	}
	return defaultFeishuBaseURL
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validUserIDType(value UserIDType) bool {
	return value == UserIDOpenID || value == UserIDUnionID || value == UserIDUserID
}

func validChatID(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "oc_") && validOpaqueID(value, 512)
}

func validOpaqueID(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum || strings.ContainsAny(value, "/\\?#") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}
