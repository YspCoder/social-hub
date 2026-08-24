// Package kakaomoment implements advertiser-bound Kakao Moment Open API v4
// management and reporting workflows.
package kakaomoment

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "kakao/moment-open-api-v4"
	platformName     = "kakao"
	productName      = "moment-open-api"
	apiVersion       = "v4"
	defaultBaseURL   = "https://apis.moment.kakao.com/openapi/v4"
	defaultAuthURL   = "https://kauth.kakao.com/oauth/business/authorize"
	defaultTokenURL  = "https://kauth.kakao.com/oauth/business/token"
	documentationURL = "https://developers.kakao.com/docs/en/kakaomoment/common"
	approvalURL      = "https://developers.kakao.com/docs/en/kakaomoment/common#how-to-use"

	ScopeManagement = "moment_management"
	ScopeDelete     = "moment_delete"
)

// AccountSettings binds one social-hub account to one Kakao Moment ad account.
type AccountSettings struct {
	AdAccountID int64 `json:"ad_account_id" yaml:"ad_account_id"`
}

// Adapter implements socialhub.Adapter for Kakao Moment Open API v4.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be moment-open-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; Kakao API and Business Authentication endpoints are fixed")
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is invalid")
		}
		if account.ClientID != "" && !validOpaque(account.ClientID, 1024) {
			return invalidArgument("init", "client_id is invalid")
		}
		if account.AccessTokenRef == "" && account.ClientID == "" {
			return invalidArgument("init", "access_token_ref or client_id is required")
		}
		if account.ClientID == "" && account.SecretRef != "" {
			return invalidArgument("init", "client_id is required when secret_ref configures a Business Authentication client secret")
		}
		if account.SecretRef != "" && !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref is invalid")
		}
		if account.AppID != "" {
			return invalidArgument("init", "app_id is not part of the Kakao Business Authentication contract")
		}
		if account.TokenStore != "" {
			return invalidArgument("init", "Business Authentication does not issue refresh tokens")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "Kakao Moment management webhooks are not supported")
		}
		if account.Approval.AccountType != "" {
			return invalidArgument("init", "approval.account_type is not part of the Kakao Moment contract")
		}
		if !validScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval scopes must contain only moment_management and moment_delete; moment_delete requires moment_management")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.AdAccountID <= 0 {
			return invalidArgument("init", "account.settings.ad_account_id must be positive")
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
	if !validOpaque(account.AccessTokenRef, 4096) {
		return nil, invalidArgument("client", "access_token_ref is required for an API client")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{
		AccessToken: accessToken, TokenType: "Bearer", Scopes: append([]string(nil), account.Approval.Scopes...),
	}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set("adAccountId", formatID(typed.AdAccountID))
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, cloneHTTPClient(resolved.HTTPClient), tokens,
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, adAccountID: typed.AdAccountID, api: api,
		scopes: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
	}, nil
}

// OAuth returns the account-bound Business Authentication authorization-code helper.
func (adapter *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	options := adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(account.ClientID, 1024) {
		return nil, invalidArgument("oauth", "client_id is required")
	}
	secret := ""
	if account.SecretRef != "" {
		var err error
		secret, err = resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AdAccountID: typed.AdAccountID,
		HTTPClient: cloneHTTPClient(options.HTTPClient),
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			PlatformMessage: "configured credential could not be resolved",
		}
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
