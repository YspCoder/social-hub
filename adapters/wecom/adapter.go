// Package wecom implements the WeCom self-built application server API.
package wecom

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName   = "wecom/corp-api"
	productName   = "corp-api"
	defaultAPIURL = "https://qyapi.weixin.qq.com"
	docURL        = "https://developer.work.weixin.qq.com/document/path/91039"
)

// Settings controls the WeCom API origin. BaseURL overrides are intended for
// deterministic tests and approved gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings identifies one self-built application and its optional
// default message recipients.
type AccountSettings struct {
	AgentID         int64    `json:"agent_id" yaml:"agent_id"`
	DefaultUserIDs  []string `json:"default_user_ids,omitempty" yaml:"default_user_ids,omitempty"`
	DefaultPartyIDs []int64  `json:"default_party_ids,omitempty" yaml:"default_party_ids,omitempty"`
	DefaultTagIDs   []int64  `json:"default_tag_ids,omitempty" yaml:"default_tag_ids,omitempty"`
}

// Adapter implements socialhub.Adapter for WeCom self-built applications.
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
		Name: adapterName, Product: productName, APIVersion: "continuous", DocURL: docURL,
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
	settings := Settings{BaseURL: defaultAPIURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		if !validCorpID(account.AppID) {
			return invalidArgument("init", "app_id must contain the WeCom CorpID for every account")
		}
		if strings.TrimSpace(account.AccessTokenRef) == "" && strings.TrimSpace(account.SecretRef) == "" {
			return invalidArgument("init", "secret_ref or access_token_ref is required for every account")
		}
		if (account.Webhook.TokenRef == "") != (account.Webhook.AESKeyRef == "") {
			return invalidArgument("init", "webhook.token_ref and webhook.aes_key_ref must be configured together")
		}
		if account.Webhook.SecretRef != "" {
			return invalidArgument("init", "webhook.secret_ref is not used; configure token_ref and aes_key_ref")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.AgentID <= 0 {
			return invalidArgument("init", "account.settings.agent_id must be positive")
		}
		if err := validateRecipientSet(RecipientSet{
			UserIDs: typed.DefaultUserIDs, PartyIDs: typed.DefaultPartyIDs, TagIDs: typed.DefaultTagIDs,
		}, true); err != nil {
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
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}

	var tokens socialhub.TokenSource
	var invalidator tokenInvalidator
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		source := &corpTokenSource{
			baseURL: settings.BaseURL, corpID: account.AppID, secret: secret,
			httpClient: resolved.HTTPClient, clock: resolved.Clock, store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: "wecom", Product: productName, Tenant: account.AppID,
				Account: string(accountID), Subject: strconv.FormatInt(typed.AgentID, 10),
			},
		}
		tokens, invalidator = source, source
	}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, resolved.HTTPClient, tokens, "wecom", productName,
		transport.QueryAuthenticator("access_token"), decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	webhookToken, err := resolveOptionalSecret(ctx, resolved.Secrets, account.Webhook.TokenRef, "client")
	if err != nil {
		return nil, err
	}
	aesKey, err := resolveOptionalSecret(ctx, resolved.Secrets, account.Webhook.AESKeyRef, "client")
	if err != nil {
		return nil, err
	}
	if webhookToken != "" {
		if len(webhookToken) > 256 {
			return nil, invalidArgument("client", "webhook token exceeds 256 bytes")
		}
		if _, err := decodeAESKey(aesKey); err != nil {
			return nil, err
		}
	}
	return &Client{
		accountID: accountID, corpID: account.AppID, agentID: typed.AgentID, api: api,
		clock: resolved.Clock, invalidator: invalidator, webhookToken: webhookToken, aesKey: aesKey,
		defaults: RecipientSet{
			UserIDs:  append([]string(nil), typed.DefaultUserIDs...),
			PartyIDs: append([]int64(nil), typed.DefaultPartyIDs...),
			TagIDs:   append([]int64(nil), typed.DefaultTagIDs...),
		},
		uploads: make(map[string]*uploadState),
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
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validCorpID(value string) bool {
	return validBoundedValue(value, 128, false)
}

var _ socialhub.Adapter = (*Adapter)(nil)
