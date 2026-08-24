// Package bitbucket implements a bounded, read-only Bitbucket Cloud REST API 2.0 adapter.
package bitbucket

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName       = "bitbucket/cloud-rest-api-v2"
	platformName      = "bitbucket"
	productName       = "cloud-rest-api"
	apiVersion        = "2.0"
	defaultBaseURL    = "https://api.bitbucket.org/2.0"
	defaultUserAgent  = "social-hub/bitbucket"
	documentationURL  = "https://developer.atlassian.com/cloud/bitbucket/rest/"
	authenticationURL = "https://developer.atlassian.com/cloud/bitbucket/rest/intro/#authentication"
)

// AuthMode selects one of Bitbucket Cloud's current API authentication forms.
type AuthMode string

const (
	AuthBearer        AuthMode = "bearer"
	AuthBasicAPIToken AuthMode = "basic_api_token"
)

// Settings contains adapter-wide request settings. The API origin is fixed to
// api.bitbucket.org so credentials cannot be redirected through configuration.
type Settings struct {
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// AccountSettings selects authentication for one configured account. Email is
// required only for Basic authentication with an Atlassian API token.
type AccountSettings struct {
	AuthMode AuthMode `json:"auth_mode" yaml:"auth_mode"`
	Email    string   `json:"email,omitempty" yaml:"email,omitempty"`
}

// Adapter implements socialhub.Adapter for Bitbucket Cloud REST API 2.0.
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
		return invalidArgument("init", "product must be cloud-rest-api")
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
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "account.access_token_ref is required")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, app_id, secret_ref, and token_store are outside this externally supplied credential contract")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are outside this read-only adapter")
		}
		if !validOptionalOpaque(account.Approval.AccountType, 256) || !validStringSet(account.Approval.Scopes, 256) {
			return invalidArgument("init", "approval metadata is invalid")
		}
		var accountSettings AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &accountSettings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validAccountSettings(accountSettings) {
			return invalidArgument("init", "account.settings must select bearer, or basic_api_token with an Atlassian email")
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
	var accountSettings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &accountSettings); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	credential, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || !validCredential(credential) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		switch accountSettings.AuthMode {
		case AuthBearer:
			request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		case AuthBasicAPIToken:
			request.SetBasicAuth(accountSettings.Email, token.AccessToken)
		default:
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("User-Agent", settings.UserAgent)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: credential}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock, credential),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, scopes: append([]string(nil), account.Approval.Scopes...),
		api: api, clock: resolved.Clock,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
