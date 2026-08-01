// Package discord implements the Discord HTTP API v10 bot adapter.
package discord

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
	adapterName      = "discord/v10"
	defaultAPIURL    = "https://discord.com/api/v10"
	defaultCDNURL    = "https://cdn.discordapp.com"
	defaultUserAgent = "DiscordBot (https://github.com/YspCoder/social-hub, dev)"
	docURL           = "https://docs.discord.com/developers/reference"
)

// CapabilityGateway identifies Discord Gateway discovery. The adapter does
// not own a persistent Gateway connection.
const CapabilityGateway socialhub.Capability = "gateway_discovery"

// Settings controls Discord HTTP endpoints and the required User-Agent.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	CDNURL    string `json:"cdn_url,omitempty" yaml:"cdn_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

type accountSettings struct {
	DefaultChannelID string `json:"default_channel_id,omitempty" yaml:"default_channel_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Discord bots.
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
		Name: adapterName, Product: "bot-api", APIVersion: "v10", DocURL: docURL,
		VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultAPIURL, CDNURL: defaultCDNURL, UserAgent: defaultUserAgent}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.CDNURL) || strings.TrimSpace(settings.UserAgent) == "" {
		return invalidArgument("init", "base_url, cdn_url, and user_agent must be valid")
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef == "" {
			return invalidArgument("init", "access_token_ref (bot token) is required for every account")
		}
		var typed accountSettings
		if len(account.Settings) > 0 {
			if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
				return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
		}
		if typed.DefaultChannelID != "" && !validSnowflake(typed.DefaultChannelID) {
			return invalidArgument("init", "default_channel_id must be a Discord snowflake")
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return wrapError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config = config
	a.options = resolved
	a.settings = settings
	a.ready = true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, wrapError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions := a.options
	settings := a.settings
	a.mu.RUnlock()
	if !found {
		return nil, wrapError("client", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	combined := []socialhub.Option{
		socialhub.WithHTTPClient(baseOptions.HTTPClient),
		socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets),
		socialhub.WithClock(baseOptions.Clock),
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	botToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || strings.TrimSpace(botToken) == "" {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: botToken, TokenType: "Bot"}}
	httpTransport, err := transport.New(settings.BaseURL, resolved.HTTPClient, tokens, "discord", "bot-api", decodeHTTPError)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	var typed accountSettings
	if len(account.Settings) > 0 {
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	client := &Client{
		accountID: accountID, transport: httpTransport, cdnURL: strings.TrimRight(settings.CDNURL, "/"),
		userAgent: settings.UserAgent, defaultChannelID: typed.DefaultChannelID,
	}
	client.workflow = &BotService{client: client}
	return client, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = false
	a.closed = true
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}
