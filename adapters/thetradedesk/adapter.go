// Package thetradedesk implements advertiser-scoped The Trade Desk Platform API v3 workflows.
// Paid-media resources remain separate from social-hub's organic interfaces.
package thetradedesk

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName          = "thetradedesk/platform-api-v3"
	platformName         = "thetradedesk"
	productName          = "platform-api"
	apiVersion           = "v3"
	productionBaseURL    = "https://api.thetradedesk.com"
	sandboxBaseURL       = "https://ext-api.sb.thetradedesk.com"
	documentationURL     = "https://partner.thetradedesk.com/v3/portal/api/overview"
	accountSetupURL      = "https://partner.thetradedesk.com/v3/portal/api/doc/ApiPlatformGetStarted"
	authenticationDocURL = "https://partner.thetradedesk.com/v3/portal/api/doc/PlatformAuthentication"
)

const defaultTokenExpirationMinutes int32 = 1440

// Settings selects the official production or Partner Sandbox REST origin.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings binds one social-hub account to one TTD advertiser.
type AccountSettings struct {
	AdvertiserID             string `json:"advertiser_id" yaml:"advertiser_id"`
	TokenExpirationInMinutes int32  `json:"token_expiration_minutes,omitempty" yaml:"token_expiration_minutes,omitempty"`
	StrictMode               *bool  `json:"strict_mode,omitempty" yaml:"strict_mode,omitempty"`
}

// Adapter implements socialhub.Adapter for The Trade Desk Platform API v3.
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
		return invalidArgument("init", "product must be platform-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: productionBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "settings.base_url must be the official The Trade Desk production or Partner Sandbox REST origin")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validID(typed.AdvertiserID) {
			return invalidArgument("init", "account.settings.advertiser_id is invalid")
		}
		if typed.TokenExpirationInMinutes < 0 || typed.TokenExpirationInMinutes > 1440 {
			return invalidArgument("init", "account.settings.token_expiration_minutes must be between 1 and 1440 when specified")
		}
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is invalid")
		}
		if account.ClientID != "" && !validOpaque(account.ClientID, 512) {
			return invalidArgument("init", "client_id for the Platform API login is invalid")
		}
		if account.SecretRef != "" && !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the Platform API password is invalid")
		}
		staticToken := account.AccessTokenRef != ""
		managedToken := account.ClientID != "" && account.SecretRef != ""
		if staticToken == managedToken {
			return invalidArgument("init", "configure exactly one of access_token_ref or client_id with secret_ref")
		}
		if !managedToken && (account.ClientID != "" || account.SecretRef != "") {
			return invalidArgument("init", "client_id and secret_ref must be configured together")
		}
		if staticToken && (account.ClientID != "" || account.SecretRef != "") {
			return invalidArgument("init", "client_id and secret_ref cannot be combined with access_token_ref")
		}
		if staticToken && typed.TokenExpirationInMinutes != 0 {
			return invalidArgument("init", "token_expiration_minutes applies only to managed short-lived tokens")
		}
		if account.AppID != "" {
			return invalidArgument("init", "app_id is not part of Platform API authentication")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by the Platform API adapter")
		}
		if len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "Platform API permissions are account grants, not OAuth scopes")
		}
		if account.Approval.AccountType != "" && account.Approval.AccountType != productName {
			return invalidArgument("init", "approval.account_type must be platform-api when specified")
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
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	requestIDs := newRequestIDFilter(
		account.ClientID, account.SecretRef, account.AccessTokenRef,
		string(accountID), typed.AdvertiserID,
	)
	var tokens closableTokenSource
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			requestIDs.clear()
			return nil, err
		}
		requestIDs.add(accessToken)
		tokens = &staticTokenSource{
			token: socialhub.Token{AccessToken: accessToken}, requestIDs: requestIDs,
		}
	} else {
		password, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			requestIDs.clear()
			return nil, err
		}
		requestIDs.add(password)
		expiration := normalizedTokenExpiration(typed.TokenExpirationInMinutes)
		authentication := AuthenticationClient{
			Login: account.ClientID, Password: password, BaseURL: settings.BaseURL,
			HTTPClient: httpClient, Clock: resolved.Clock, requestIDs: requestIDs,
		}
		tokens = &authenticationTokenSource{
			authentication: authentication, expirationMinutes: expiration,
			store: resolved.TokenStore, requestIDs: requestIDs,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName,
				Account: string(accountID), Subject: typed.AdvertiserID,
			},
		}
	}
	authenticator := requestAuthenticator{strictMode: typed.StrictMode}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, httpClient, tokens, platformName, productName,
		authenticator, newHTTPErrorDecoder(resolved.Clock, requestIDs),
	)
	if err != nil {
		tokens.Close()
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, advertiserID: typed.AdvertiserID, api: api,
		approval: account.Approval, managedAuth: account.AccessTokenRef == "",
		tokens: tokens, requestIDs: requestIDs,
	}, nil
}

// Authentication returns the short-lived token helper for a managed-login account.
func (adapter *Adapter) Authentication(ctx context.Context, accountID socialhub.AccountID) (*AuthenticationClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("authentication", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	settings, options := adapter.settings, adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("authentication", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if account.AccessTokenRef != "" {
		return nil, invalidArgument("authentication", "short-lived token generation is unavailable for a static-token account")
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("authentication", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	password, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "authentication")
	if err != nil {
		return nil, err
	}
	requestIDs := newRequestIDFilter(
		account.ClientID, password, account.SecretRef, string(accountID), typed.AdvertiserID,
	)
	return &AuthenticationClient{
		Login: account.ClientID, Password: password, BaseURL: settings.BaseURL,
		HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
		requestIDs: requestIDs,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, errCredentialResolution)
	}
	if !validOpaque(value, 16_384) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	return &copy
}

func normalizedTokenExpiration(value int32) int32 {
	if value == 0 {
		return defaultTokenExpirationMinutes
	}
	return value
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
