// Package xiaomiglobalreporting implements Xiaomi Global's read-only Mi Ads
// Reporting API and its token creation and refresh endpoints.
package xiaomiglobalreporting

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
	adapterName      = "xiaomi/global-reporting-api"
	platformName     = "xiaomi"
	productName      = "global-reporting-api"
	apiVersion       = "unversioned-2026-08"
	baseURL          = "https://global.e.mi.com"
	documentationURL = "https://global.e.mi.com/doc/reporting_api_guide.html"
)

// Settings selects the unit for the timestamp Cookie. Xiaomi documents the
// value as Long and requires it to change on every request, but does not
// publish its unit.
type Settings struct {
	TimestampUnit TimestampUnit `json:"timestamp_unit" yaml:"timestamp_unit"`
}

// AccountSettings restricts one social-hub account to an explicit set of
// Reporting API account IDs granted to the external Xiaomi application.
type AccountSettings struct {
	AccountIDs []int64 `json:"account_ids" yaml:"account_ids"`
}

// Adapter implements socialhub.Adapter for Xiaomi Global Reporting API.
type Adapter struct {
	mu         sync.RWMutex
	config     socialhub.AdapterConfig
	options    socialhub.Options
	settings   Settings
	timestamps *timestampSequence
	ready      bool
	closed     bool
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
		return invalidArgument("init", "product must be global-reporting-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	var settings Settings
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if settings.TimestampUnit != TimestampUnixSeconds && settings.TimestampUnit != TimestampUnixMilliseconds {
		return invalidArgument("init", "settings.timestamp_unit must explicitly be unix_seconds or unix_milliseconds")
	}
	for _, account := range config.Accounts {
		if !validCookieValueReference(account.AccessTokenRef) {
			return invalidArgument("init", "access_token_ref is required for every Xiaomi Global account")
		}
		if (strings.TrimSpace(account.AppID) == "") != (strings.TrimSpace(account.SecretRef) == "") {
			return invalidArgument("init", "app_id and secret_ref must be configured together when token helpers are used")
		}
		if account.AppID != "" && !validOpaque(account.AppID, 256) {
			return invalidArgument("init", "app_id is invalid")
		}
		if account.ClientID != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id and token_store are not used by this adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by the Xiaomi Global reporting adapter")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validIDList(typed.AccountIDs, maximumNameIDs, false) {
			return invalidArgument("init", "account.settings.account_ids must contain unique positive authorized account IDs")
		}
	}

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	adapter.config = config
	adapter.options = resolved
	adapter.settings = settings
	adapter.timestamps = &timestampSequence{unit: settings.TimestampUnit}
	adapter.ready = true
	return nil
}

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	baseOptions, timestamps := adapter.options, adapter.timestamps
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
	accessToken, err := resolveAccessToken(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	authorized := make(map[int64]struct{}, len(typed.AccountIDs))
	for _, id := range typed.AccountIDs {
		authorized[id] = struct{}{}
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.AddCookie(&http.Cookie{Name: "access_token", Value: token.AccessToken})
		return nil
	})
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "MiAdsCookie"}}
	api, err := transport.NewWithAuthenticator(
		baseURL, httpClient, tokens, platformName, productName, authenticator,
		newHTTPErrorDecoder(resolved.Clock, accessToken),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, clock: resolved.Clock, timestamps: timestamps,
		authorizedAccountIDs: append([]int64(nil), typed.AccountIDs...), authorizedAccounts: authorized,
		redactionSecrets: []string{accessToken},
	}, nil
}

// Tokens returns Xiaomi's appId/appKey token creation and refresh helper. The
// helper returns credentials to the caller and never persists them.
func (adapter *Adapter) Tokens(ctx context.Context, accountID socialhub.AccountID) (*TokenClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("tokens", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	options := adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("tokens", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(account.AppID, 256) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("tokens", "app_id and secret_ref are required for token management")
	}
	appKey, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "tokens")
	if err != nil {
		return nil, err
	}
	return &TokenClient{
		appID: account.AppID, appKey: appKey,
		httpClient: cloneHTTPClient(options.HTTPClient), clock: options.Clock,
	}, nil
}

func resolveAccessToken(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", credentialError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err, reference)
	}
	if !validCookieValue(value) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	return value, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", credentialError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err, reference)
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

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
