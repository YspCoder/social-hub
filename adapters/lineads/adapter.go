// Package lineads implements the restricted LINE Ads API v3 read surface.
package lineads

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "line/ads-api-v3"
	platformName     = "line"
	productName      = "ads-api"
	apiVersion       = "3.12.3"
	defaultBaseURL   = "https://ads.line.me/api"
	documentationURL = "https://ads.line.me/public-docs/pages/v3/3.12.3/reporting-general-partner/"
)

// AccountSettings identifies the API-enabled LINE Ads group.
type AccountSettings struct {
	GroupID string `json:"group_id" yaml:"group_id"`
}

// Adapter implements socialhub.Adapter for LINE Ads API v3.
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
		return invalidArgument("init", "product must be ads-api")
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter-level settings are unsupported; the LINE Ads API origin is fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 256) || !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "client_id (Access key) and secret_ref (Secret key reference) are required")
		}
		if account.AppID != "" || account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "app_id, access_token_ref, and token_store are not used by LINE Ads JWS authentication")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by these read workflows")
		}
		if len(account.Approval.Scopes) > 0 || !validPartnerType(PartnerType(account.Approval.AccountType)) {
			return invalidArgument("init", "approval.account_type must be an official LINE Ads partner type and approval.scopes must be empty")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validPathSegment(typed.GroupID, 256) {
			return invalidArgument("init", "account.settings.group_id is required and must be a safe path segment")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	secretKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: secretKey, TokenType: "JWS-HS256"}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		return authenticateRequest(request, account.ClientID, token.AccessToken, resolved.Clock)
	})
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, tokens, platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, groupID: typed.GroupID,
		partnerType: PartnerType(account.Approval.AccountType), api: api,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the LINE Ads Secret key", err, reference, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved LINE Ads Secret key is invalid", nil, reference, value)
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
