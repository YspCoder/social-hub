// Package slack implements Slack Web API and Events API adapters.
package slack

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
	adapterName    = "slack/web-api"
	productName    = "web-api"
	apiVersion     = "unversioned"
	defaultBaseURL = "https://slack.com/api"
	docURL         = "https://docs.slack.dev/apis/web-api/"
)

// TokenKind identifies the Slack installation token actor.
type TokenKind string

const (
	TokenBot  TokenKind = "bot"
	TokenUser TokenKind = "user"
)

// Settings controls the Slack Web API origin. BaseURL overrides are intended
// for deterministic contract tests, GovSlack, and approved gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings identifies one Slack workspace installation.
type AccountSettings struct {
	WorkspaceID      string    `json:"workspace_id" yaml:"workspace_id"`
	TokenKind        TokenKind `json:"token_kind" yaml:"token_kind"`
	ActorID          string    `json:"actor_id,omitempty" yaml:"actor_id,omitempty"`
	DefaultChannelID string    `json:"default_channel_id,omitempty" yaml:"default_channel_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Slack workspace installations.
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
			return invalidArgument("init", "access_token_ref is required for every Slack account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validSlackID(typed.WorkspaceID, "T") {
			return invalidArgument("init", "account.settings.workspace_id must be a Slack team ID")
		}
		if typed.TokenKind != TokenBot && typed.TokenKind != TokenUser {
			return invalidArgument("init", "account.settings.token_kind must be bot or user")
		}
		if typed.ActorID != "" && !validSlackID(typed.ActorID, "UWB") {
			return invalidArgument("init", "account.settings.actor_id must be a Slack user or bot ID")
		}
		if typed.DefaultChannelID != "" && !validSlackID(typed.DefaultChannelID, "CGD") {
			return invalidArgument("init", "account.settings.default_channel_id must be a Slack conversation ID")
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
	signingSecret := ""
	if account.Webhook.SecretRef != "" {
		signingSecret, err = resolveSecret(ctx, resolved.Secrets, account.Webhook.SecretRef, "client")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	apiTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "slack", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	return &Client{
		accountID: accountID, workspaceID: typed.WorkspaceID, tokenKind: typed.TokenKind,
		actorID: typed.ActorID, defaultChannelID: typed.DefaultChannelID,
		api: apiTransport, httpClient: resolved.HTTPClient, clock: resolved.Clock,
		signingSecret: signingSecret, scopes: append([]string(nil), account.Approval.Scopes...),
		allowHTTPUploads: baseURL.Scheme == "http", uploads: make(map[string]*uploadState), media: make(map[string]socialhub.Media),
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

func validSlackID(value, prefixes string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 2 || len(value) > 255 || !strings.ContainsRune(prefixes, rune(value[0])) {
		return false
	}
	for _, character := range value[1:] {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}
