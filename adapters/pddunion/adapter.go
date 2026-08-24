// Package pddunion implements Duoduo Jinbao affiliate workflows for
// Pinduoduo publishers. Affiliate goods and attributed orders intentionally
// remain separate from social-hub's organic post model.
package pddunion

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "pinduoduo/duoduo-jinbao-api-v1"
	platformName     = "pinduoduo"
	productName      = "duoduo-jinbao-api"
	apiVersion       = "V1"
	defaultBaseURL   = "https://gw-api.pinduoduo.com/api/router"
	documentationURL = "https://open.pinduoduo.com/application/document/api?id=pdd.ddk.goods.recommend.get"
	publisherType    = "duoduo-jinbao-publisher"
)

// AccountSettings contains publisher-specific Duoduo Jinbao parameters.
type AccountSettings struct {
	DefaultPID string `json:"default_pid,omitempty" yaml:"default_pid,omitempty"`
}

// Adapter implements socialhub.Adapter for the Duoduo Jinbao V1 gateway.
type Adapter struct {
	mu      sync.RWMutex
	config  socialhub.AdapterConfig
	options socialhub.Options
	ready   bool
	closed  bool
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
		return invalidArgument("init", "product must be duoduo-jinbao-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 256) {
			return invalidArgument("init", "client_id must contain the Pinduoduo client_id")
		}
		if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the Pinduoduo client_secret is required")
		}
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref for the optional Pinduoduo access_token is invalid")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id, token_store, and webhook settings are not used by this adapter")
		}
		if (account.Approval.AccountType != "" && account.Approval.AccountType != publisherType) || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "approval.account_type may only be duoduo-jinbao-publisher and OAuth scopes are not used")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.DefaultPID != "" && !validPID(typed.DefaultPID) {
			return invalidArgument("init", "account.settings.default_pid is invalid")
		}
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	adapter.config, adapter.options, adapter.ready = config, resolved, true
	return nil
}

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	baseOptions := adapter.options
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
	clientSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	var accessToken string
	if account.AccessTokenRef != "" {
		accessToken, err = resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	origin, path, err := splitGatewayURL(defaultBaseURL)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	authenticator := &pddAuthenticator{
		clientID: account.ClientID, accessToken: accessToken, clock: resolved.Clock,
	}
	api, err := transport.NewWithAuthenticator(
		origin, cloneHTTPClient(resolved.HTTPClient),
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: clientSecret}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, defaultPID: typed.DefaultPID, gatewayPath: path,
		api: api, approval: account.Approval, clock: resolved.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Pinduoduo credential", err, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved Pinduoduo credential is invalid", nil, value)
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
