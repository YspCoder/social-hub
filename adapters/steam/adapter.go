// Package steam implements a bounded, read-only subset of the Steam Web API.
package steam

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "steam/web-api"
	platformName     = "steam"
	productName      = "web-api"
	apiVersion       = "v2"
	defaultBaseURL   = "https://api.steampowered.com"
	documentationURL = "https://partner.steamgames.com/doc/webapi"
	userKeyURL       = "https://steamcommunity.com/dev/apikey"
)

// Adapter implements socialhub.Adapter for the public Steam Web API host.
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
		return invalidArgument("init", "product must be web-api")
	}
	if len(config.Settings) > 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if account.AccessTokenRef != "" && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "account.access_token_ref is invalid")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, app_id, secret_ref, and token_store are outside the Steam Web API key contract")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are outside this read-only adapter")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth approval metadata is outside the Steam Web API key contract")
		}
		if len(account.Settings) > 0 {
			var empty struct{}
			if err := socialhub.DecodeSettings(account.Settings, &empty); err != nil {
				return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
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

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	publicAPI, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "anonymous"}},
		platformName, productName,
		transport.AuthenticatorFunc(func(*http.Request, socialhub.Token) error { return nil }),
		newHTTPErrorDecoder(resolved.Clock, ""),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}

	var authenticatedAPI *transport.Client
	var webAPIKey string
	if account.AccessTokenRef != "" {
		webAPIKey, err = resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
		if err != nil {
			return nil, authenticationError("client", err, webAPIKey)
		}
		if !validWebAPIKey(webAPIKey) {
			return nil, authenticationError("client", nil, webAPIKey)
		}
		authenticatedAPI, err = transport.NewWithAuthenticator(
			defaultBaseURL, &httpClient,
			socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: webAPIKey}},
			platformName, productName,
			transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
				request.Header.Set("x-webapi-key", token.AccessToken)
				return nil
			}),
			newHTTPErrorDecoder(resolved.Clock, webAPIKey),
		)
		if err != nil {
			return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	return &Client{
		accountID: accountID, authenticated: authenticatedAPI, public: publicAPI,
		webAPIKey: webAPIKey, clock: resolved.Clock,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
