// Package moloco implements the read-only Moloco Ads Report API v1.10.
package moloco

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
	adapterName       = "moloco/ads-campaign-management-v1.10"
	platformName      = "moloco"
	productName       = "ads-campaign-management"
	apiVersion        = "v1.10"
	defaultBaseURL    = "https://api.moloco.cloud"
	documentationURL  = "https://developer.moloco.cloud/docs/report-api"
	gettingStartedURL = "https://developer.moloco.cloud/docs/getting-started"
)

// AccountSettings binds one social-hub account to a Moloco Ads ad account. The API
// key referenced by SecretRef determines the workplace.
type AccountSettings struct {
	AdAccountID string `json:"ad_account_id" yaml:"ad_account_id"`
}

// Adapter implements socialhub.Adapter for the Moloco Ads Report API.
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
		return invalidArgument("init", "product must be ads-campaign-management")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter settings are not supported; the official Moloco endpoint is fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		var settings AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validIdentifier(settings.AdAccountID) {
			return invalidArgument("init", "account.settings.ad_account_id is invalid")
		}
		if strings.TrimSpace(account.SecretRef) == "" || account.ClientID != "" || account.AppID != "" ||
			account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "configure only secret_ref with the Moloco Ads API key")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by the Report API")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "OAuth approval fields are not used by the Moloco API-key flow")
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
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	apiKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef)
	if err != nil {
		return nil, err
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	tokens := &tokenSource{apiKey: apiKey, httpClient: httpClient, clock: resolved.Clock}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validSecret(token.AccessToken) {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set("Moloco-Cloud-Api-Version", apiVersion)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, tokens, platformName, productName, authenticator,
		newHTTPErrorDecoder(resolved.Clock, tokens.redactionSecrets),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, adAccountID: settings.AdAccountID,
		api: api, tokens: tokens,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validSecret(value) {
		return "", platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
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
