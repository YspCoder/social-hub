// Package mailchimp implements a privacy-bounded, read-only Mailchimp
// Marketing API 3.0 adapter.
package mailchimp

import (
	"context"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName                = "mailchimp/marketing-api-v3.0"
	platformName               = "mailchimp"
	productName                = "marketing-api"
	apiVersion                 = "3.0.91"
	apiPathVersion             = "3.0"
	defaultUserAgent           = "social-hub/mailchimp"
	basicUsername              = "social-hub"
	documentationURL           = "https://mailchimp.com/developer/marketing/docs/fundamentals/"
	DocumentedConcurrencyLimit = 10
)

// Settings contains non-secret request metadata. The endpoint is deliberately
// not configurable because it is derived per account from a data center.
type Settings struct {
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings optionally supplies the account data center when an API key
// does not carry the usual key-dc suffix.
type AccountSettings struct {
	DataCenter string `json:"data_center,omitempty" yaml:"data_center,omitempty"`
}

// Adapter implements socialhub.Adapter for Mailchimp Marketing API 3.0.
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

func (adapter *Adapter) Name() string { return adapterName }

func (adapter *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name: adapterName, Product: productName, APIVersion: apiVersion,
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if config.Product != "" && config.Product != productName {
		return invalidArgument("init", "product must be marketing-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{UserAgent: defaultUserAgent}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validOpaque(settings.UserAgent, 256) {
		return invalidArgument("init", "user_agent must be a valid non-empty value")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "account.secret_ref for a Mailchimp API key is required")
		}
		if account.ClientID != "" || account.AppID != "" || account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, app_id, access_token_ref, and token_store are outside this API-key contract")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are outside this read-only adapter")
		}
		if (account.Approval.AccountType != "" && account.Approval.AccountType != "api_key") || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "approval must use account_type api_key without OAuth scopes")
		}
		var typed AccountSettings
		if len(account.Settings) > 0 {
			if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
				return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
		}
		if typed.DataCenter != "" && !validDataCenter(typed.DataCenter) {
			return invalidArgument("init", "account.settings.data_center must match a Mailchimp us<number> data center")
		}
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	adapter.config, adapter.options, adapter.settings, adapter.ready = config, resolved, settings, true
	return nil
}

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	baseOptions, settings := adapter.options, adapter.settings
	adapter.mu.RUnlock()
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
	apiKey, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || !validAPIKey(apiKey) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if len(account.Settings) > 0 {
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	dataCenter, err := resolveDataCenter(apiKey, typed.DataCenter)
	if err != nil {
		return nil, invalidArgument("client", err.Error())
	}
	baseURL := "https://" + dataCenter + ".api.mailchimp.com/" + apiPathVersion
	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(basicUsername+":"+apiKey))
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validAPIKey(token.AccessToken) {
			return socialhub.ErrUnauthenticated
		}
		request.SetBasicAuth(basicUsername, token.AccessToken)
		request.Header.Set("User-Agent", settings.UserAgent)
		return nil
	})
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	api, err := transport.NewWithAuthenticator(
		baseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: apiKey, TokenType: "Basic"}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock, apiKey, authorization),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, apiKey: apiKey,
		authorization: authorization, approval: account.Approval, clock: resolved.Clock,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
