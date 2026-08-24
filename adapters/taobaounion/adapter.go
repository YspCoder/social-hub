// Package taobaounion implements Alibaba TOP v2 workflows for Taobao Union
// publishers. Affiliate materials and attributed orders intentionally remain
// separate from social-hub's organic post model.
package taobaounion

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "alimama/taobao-union-api-v2"
	platformName     = "alimama"
	productName      = "taobao-union-api"
	apiVersion       = "2.0"
	defaultBaseURL   = "https://eco.taobao.com/router/rest"
	sandboxBaseURL   = "https://gw.api.tbsandbox.com/router/rest"
	documentationURL = "https://open.taobao.com/API.htm?docType=2&docId=48340"
	publisherType    = "taobao-union-publisher"
)

// Settings selects the official TOP production or sandbox gateway.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings contains publisher-specific TOP parameters.
type AccountSettings struct {
	DefaultAdzoneID string `json:"default_adzone_id,omitempty" yaml:"default_adzone_id,omitempty"`
	PartnerID       string `json:"partner_id,omitempty" yaml:"partner_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Taobao Union API v2.
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
		return invalidArgument("init", "product must be taobao-union-api")
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
	if !validGatewayURL(settings.BaseURL) {
		return invalidArgument("init", "base_url must be the official TOP production or sandbox gateway")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 256) {
			return invalidArgument("init", "client_id must contain the TOP app_key")
		}
		if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the TOP app_secret is required")
		}
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref for the optional TOP session is invalid")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id, token_store, and webhook settings are not used by this adapter")
		}
		if (account.Approval.AccountType != "" && account.Approval.AccountType != publisherType) || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "approval.account_type may only be taobao-union-publisher and OAuth scopes are not used")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.DefaultAdzoneID != "" && !validNumericID(typed.DefaultAdzoneID, 20) {
			return invalidArgument("init", "account.settings.default_adzone_id must be a positive numeric promotion-position ID")
		}
		if typed.PartnerID != "" && !validOpaque(typed.PartnerID, 256) {
			return invalidArgument("init", "account.settings.partner_id is invalid")
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
	appSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	var session string
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		session, err = resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	origin, path, err := splitGatewayURL(settings.BaseURL)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	authenticator := &topAuthenticator{
		appKey: account.ClientID, session: session, partnerID: typed.PartnerID, clock: resolved.Clock,
	}
	api, err := transport.NewWithAuthenticator(
		origin, cloneHTTPClient(resolved.HTTPClient),
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: appSecret}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, defaultAdzoneID: typed.DefaultAdzoneID, gatewayPath: path,
		api: api, approval: account.Approval, clock: resolved.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the TOP credential", err, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved TOP credential is invalid", nil, value)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	return &copy
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
