// Package telegram implements the Telegram Bot API adapter.
package telegram

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"

	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "telegram/bot-api"
	defaultAPIURL    = "https://api.telegram.org"
	docURL           = "https://core.telegram.org/bots/api"
	maxResponseBytes = 8 << 20
)

// CapabilityMediaSend identifies Telegram's sendPhoto/sendVideo/sendDocument APIs.
const CapabilityMediaSend socialhub.Capability = "media_send"

// Settings controls the Telegram Bot API origin.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

type accountSettings struct {
	DefaultChatID string `json:"default_chat_id,omitempty" yaml:"default_chat_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Telegram Bot API.
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
		Name:       adapterName,
		Product:    "bot-api",
		APIVersion: "10.2",
		DocURL:     docURL,
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
	settings := Settings{BaseURL: defaultAPIURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return wrapError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.BaseURL == "" {
		return invalidArgument("init", "base_url is required")
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
	if err != nil {
		return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var webhookSecret string
	if account.Webhook.SecretRef != "" {
		webhookSecret, err = resolved.Secrets.Resolve(ctx, account.Webhook.SecretRef)
		if err != nil {
			return nil, wrapError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
		if !validWebhookSecret(webhookSecret) {
			return nil, invalidArgument("client", "webhook secret must be 1-256 characters using A-Z, a-z, 0-9, _ or -")
		}
	}
	var typed accountSettings
	if len(account.Settings) > 0 {
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	telegramBot, err := tgbot.New(
		botToken,
		tgbot.WithServerURL(settings.BaseURL),
		tgbot.WithHTTPClient(time.Minute, boundedHTTPClient{client: resolved.HTTPClient, maximum: maxResponseBytes}),
		tgbot.WithSkipGetMe(),
		tgbot.WithErrorsHandler(func(error) {}),
	)
	if err != nil {
		return nil, wrapError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	client := &Client{
		accountID: accountID, bot: telegramBot, clock: resolved.Clock,
		defaultChatID: typed.DefaultChatID, webhookSecret: webhookSecret,
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

var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

func validWebhookSecret(value string) bool { return webhookSecretPattern.MatchString(value) }

type boundedHTTPClient struct {
	client  *http.Client
	maximum int64
}

func (c boundedHTTPClient) Do(request *http.Request) (*http.Response, error) {
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.ContentLength > c.maximum {
		_ = response.Body.Close()
		return nil, fmt.Errorf("%w: maximum %d bytes", errResponseTooLarge, c.maximum)
	}
	response.Body = &boundedReadCloser{Reader: &limitErrorReader{reader: response.Body, maximum: c.maximum}, Closer: response.Body}
	return response, nil
}
