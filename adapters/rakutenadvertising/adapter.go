// Package rakutenadvertising implements publisher-facing Rakuten Advertising
// Affiliate APIs workflows.
package rakutenadvertising

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
	adapterName      = "rakuten-advertising/affiliate-apis-v1.0.0"
	platformName     = "rakuten-advertising"
	productName      = "affiliate-apis"
	apiVersion       = "1.0.0"
	defaultBaseURL   = "https://api.linksynergy.com"
	defaultTokenURL  = "https://api.linksynergy.com/token"
	documentationURL = "https://developers.rakutenadvertising.com/documentation/en-US/affiliate_apis"
)

// Settings controls Rakuten Advertising API endpoints. Overrides are intended
// for controlled contract-verification gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings binds one social-hub account to a Rakuten publisher account.
// RefreshTokenRef enables managed one-hour access-token refresh.
type AccountSettings struct {
	PublisherID     string `json:"publisher_id" yaml:"publisher_id"`
	RefreshTokenRef string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Affiliate APIs 1.0.0.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be affiliate-apis")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.TokenURL) {
		return invalidArgument("init", "base_url and token_url must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validPositiveID(typed.PublisherID) {
			return invalidArgument("init", "account.settings.publisher_id must be a positive string-encoded integer")
		}
		staticToken := validOpaque(account.AccessTokenRef, 4096)
		managedRefresh := account.AccessTokenRef == "" && validOpaque(account.ClientID, 1024) &&
			validOpaque(account.SecretRef, 4096) && validOpaque(typed.RefreshTokenRef, 4096)
		if !staticToken && !managedRefresh {
			return invalidArgument("init", "configure access_token_ref or client_id, secret_ref, and account.settings.refresh_token_ref")
		}
		if staticToken && (account.ClientID != "" || account.SecretRef != "" || typed.RefreshTokenRef != "") {
			return invalidArgument("init", "managed refresh credentials cannot be combined with access_token_ref")
		}
		if account.AppID != "" {
			return invalidArgument("init", "app_id is not used by Rakuten Advertising Affiliate APIs")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by these request/response workflows")
		}
		for _, scope := range account.Approval.Scopes {
			if !validOpaque(scope, 1024) {
				return invalidArgument("init", "approval scopes contain an invalid value")
			}
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
	var tokens socialhub.TokenSource
	var errorSecrets func() []string
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{
			AccessToken: accessToken, TokenType: "Bearer", Scopes: []string{typed.PublisherID},
		}}
		configuredSecrets := []string{accessToken}
		errorSecrets = func() []string { return append([]string(nil), configuredSecrets...) }
	} else {
		clientSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		refreshToken, err := resolveSecret(ctx, resolved.Secrets, typed.RefreshTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokenKey := base64.StdEncoding.EncodeToString([]byte(account.ClientID + ":" + clientSecret))
		source := &refreshTokenSource{
			client: tokenClient{
				TokenURL: settings.TokenURL, TokenKey: tokenKey, PublisherID: typed.PublisherID,
				HTTPClient: httpClient, Clock: resolved.Clock,
			},
			refreshToken:      refreshToken,
			configuredSecrets: []string{account.ClientID, clientSecret, refreshToken, tokenKey},
			store:             resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Subject: typed.PublisherID, Scopes: typed.PublisherID,
			},
		}
		tokens, errorSecrets = source, source.redactionSecrets
	}
	decodeError := newHTTPErrorDecoder(resolved.Clock, errorSecrets)
	api, err := transport.New(settings.BaseURL, httpClient, tokens, platformName, productName, decodeError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, publisherID: typed.PublisherID, api: api,
		httpClient: httpClient, decodeError: decodeError, errorSecrets: errorSecrets,
		approval: account.Approval, clock: resolved.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
