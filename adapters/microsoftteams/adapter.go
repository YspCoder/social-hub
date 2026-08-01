// Package microsoftteams implements Microsoft Teams messaging through Microsoft Graph v1.0.
package microsoftteams

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
	adapterName = "microsoft-teams/graph-v1"
	productName = "microsoft-graph"
	apiVersion  = "v1.0"
	docURL      = "https://learn.microsoft.com/en-us/graph/api/resources/teams-api-overview"
)

// Cloud selects a Microsoft Graph national cloud deployment.
type Cloud string

const (
	CloudGlobal Cloud = "global"
	CloudUSGov  Cloud = "usgov"
	CloudDoD    Cloud = "dod"
	CloudChina  Cloud = "china"
)

// TokenKind identifies the permission model of the supplied access token.
type TokenKind string

const (
	TokenDelegated   TokenKind = "delegated"
	TokenApplication TokenKind = "application"
)

// Settings overrides the Graph origin for deterministic tests and approved gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings identifies one Teams tenant actor and optional default target.
type AccountSettings struct {
	Cloud            Cloud     `json:"cloud" yaml:"cloud"`
	TokenKind        TokenKind `json:"token_kind" yaml:"token_kind"`
	TenantID         string    `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	ActorID          string    `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	DefaultChatID    string    `json:"default_chat_id,omitempty" yaml:"default_chat_id,omitempty"`
	DefaultTeamID    string    `json:"default_team_id,omitempty" yaml:"default_team_id,omitempty"`
	DefaultChannelID string    `json:"default_channel_id,omitempty" yaml:"default_channel_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Microsoft Teams accounts.
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
			return invalidArgument("init", "access_token_ref is required for every Microsoft Teams account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validCloud(typed.Cloud) {
			return invalidArgument("init", "account.settings.cloud must be global, usgov, dod, or china")
		}
		if typed.TokenKind != TokenDelegated && typed.TokenKind != TokenApplication {
			return invalidArgument("init", "account.settings.token_kind must be delegated or application")
		}
		if typed.TenantID != "" && !validOpaqueID(typed.TenantID, 512) {
			return invalidArgument("init", "account.settings.tenant_id must be a bounded opaque tenant ID")
		}
		if typed.ActorID != "" && !validOpaqueID(typed.ActorID, 512) {
			return invalidArgument("init", "account.settings.actor_id must be a bounded opaque actor ID")
		}
		if err := validateDefaultTarget(typed); err != nil {
			return err
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
	clientState, err := resolveOptionalSecret(ctx, resolved.Secrets, account.Webhook.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	if len(clientState) > 128 {
		return nil, invalidArgument("client", "webhook clientState must not exceed 128 bytes")
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL := settings.BaseURL
	if baseURL == "" {
		baseURL = cloudBaseURL(typed.Cloud)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	api, err := transport.New(baseURL, resolved.HTTPClient, tokens, "microsoft-teams", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	parsedBaseURL, _ := url.Parse(baseURL)
	return &Client{
		accountID: accountID, cloud: typed.Cloud, tokenKind: typed.TokenKind, tenantID: typed.TenantID,
		actorID: typed.ActorID, defaultTarget: defaultTarget(typed), api: api, baseURL: parsedBaseURL,
		clock: resolved.Clock, clientState: clientState, scopes: append([]string(nil), account.Approval.Scopes...),
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

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validCloud(cloud Cloud) bool {
	return cloud == CloudGlobal || cloud == CloudUSGov || cloud == CloudDoD || cloud == CloudChina
}

func cloudBaseURL(cloud Cloud) string {
	switch cloud {
	case CloudUSGov:
		return "https://graph.microsoft.us/v1.0"
	case CloudDoD:
		return "https://dod-graph.microsoft.us/v1.0"
	case CloudChina:
		return "https://microsoftgraph.chinacloudapi.cn/v1.0"
	default:
		return "https://graph.microsoft.com/v1.0"
	}
}

func validateDefaultTarget(settings AccountSettings) error {
	chatConfigured := strings.TrimSpace(settings.DefaultChatID) != ""
	teamConfigured := strings.TrimSpace(settings.DefaultTeamID) != ""
	channelConfigured := strings.TrimSpace(settings.DefaultChannelID) != ""
	if teamConfigured != channelConfigured {
		return invalidArgument("init", "default_team_id and default_channel_id must be configured together")
	}
	if chatConfigured && teamConfigured {
		return invalidArgument("init", "default_chat_id and the default channel target are mutually exclusive")
	}
	for _, value := range []string{settings.DefaultChatID, settings.DefaultTeamID, settings.DefaultChannelID} {
		if value != "" && !validOpaqueID(value, 2048) {
			return invalidArgument("init", "default target IDs must be bounded opaque IDs")
		}
	}
	return nil
}

func defaultTarget(settings AccountSettings) *Target {
	if settings.DefaultChatID != "" {
		return &Target{Kind: TargetChat, ChatID: settings.DefaultChatID}
	}
	if settings.DefaultTeamID != "" {
		return &Target{Kind: TargetChannel, TeamID: settings.DefaultTeamID, ChannelID: settings.DefaultChannelID}
	}
	return nil
}

func validOpaqueID(value string, maximum int) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > maximum || strings.ContainsAny(value, "/\\?#") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

var _ socialhub.Adapter = (*Adapter)(nil)
