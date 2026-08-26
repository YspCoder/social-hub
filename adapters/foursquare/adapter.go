// Package foursquare implements the current Foursquare Places API.
package foursquare

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "foursquare/places-api-2025-06-17"
	platformName     = "foursquare"
	productName      = "places-api"
	apiVersion       = "2025-06-17"
	defaultBaseURL   = "https://places-api.foursquare.com"
	documentationURL = "https://docs.foursquare.com/fsq-developers-places/"
	versionHeader    = "X-Places-Api-Version"
)

// Adapter implements socialhub.Adapter for the 2025-06-17 Places API.
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
		return invalidArgument("init", "product must be places-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; Places API uses the fixed official HTTPS origin")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) || account.ClientID != "" || account.SecretRef != "" ||
			account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "access_token_ref for a Foursquare service key is required; client_id, secret_ref, app_id, and token_store are not used")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by the Places API read workflows")
		}
		var empty struct{}
		if err := socialhub.DecodeSettings(account.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
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
	serviceKey, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || !validOpaque(serviceKey, 16_384) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set(versionHeader, apiVersion)
		return nil
	})
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: serviceKey, TokenType: "Bearer"}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock, serviceKey),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api}, nil
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
