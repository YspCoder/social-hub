// Package pixabay implements bounded, read-only Pixabay image and video
// search workflows.
package pixabay

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "pixabay/api"
	platformName     = "pixabay"
	productName      = "api"
	apiVersion       = "unversioned"
	baseURL          = "https://pixabay.com/api"
	defaultUserAgent = "social-hub/pixabay"
	documentationURL = "https://pixabay.com/api/docs/"
	termsURL         = "https://pixabay.com/service/terms/"
)

// Adapter implements socialhub.Adapter for the Pixabay API.
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
		return invalidArgument("init", "product must be api")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter settings are not supported; the official Pixabay HTTPS origin is fixed")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "account.access_token_ref is required for the Pixabay API key")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, app_id, secret_ref, and token_store are not used by Pixabay API key authentication")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by Pixabay search")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "account type and scopes are not used by Pixabay API keys")
		}
		if len(account.Settings) != 0 {
			return invalidArgument("init", "account settings are not supported")
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
	apiKey, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || !validAPIKey(apiKey) {
		return nil, authenticationError("client", err)
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		query := request.URL.Query()
		query.Set("key", token.AccessToken)
		request.URL.RawQuery = query.Encode()
		request.Header.Set("User-Agent", defaultUserAgent)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		baseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: apiKey}},
		platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock, apiKey),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api, apiKey: apiKey}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
