// Package googlebooks implements the public Google Books API v1 Volume reads.
package googlebooks

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName       = "google/books-api-v1"
	platformName      = "google-books"
	productName       = "books-api"
	apiVersion        = "v1"
	discoveryRevision = "20260818"
	baseURL           = "https://books.googleapis.com"
	documentationURL  = "https://developers.google.com/books/docs/v1/using"
	discoveryURL      = "https://books.googleapis.com/$discovery/rest?version=v1"
	authorizationURL  = "https://developers.google.com/books/docs/v1/using#auth"
	volumeListURL     = "https://developers.google.com/books/docs/v1/reference/volumes/list"
	volumeGetURL      = "https://developers.google.com/books/docs/v1/reference/volumes/get"
	ScopeBooks        = "https://www.googleapis.com/auth/books"
)

// Adapter implements socialhub.Adapter for one or more Google API projects.
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
		return invalidArgument("init", "product must be books-api")
	}
	if len(config.Settings) != 0 {
		return invalidArgument("init", "adapter settings are not supported; Discovery fixes the production HTTPS origin")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if account.SecretRef == "" && account.AccessTokenRef == "" {
			return invalidArgument("init", "account.secret_ref for an API key or access_token_ref for OAuth is required")
		}
		if !validOptionalSensitive(account.SecretRef, maxSecretReferenceLength) ||
			!validOptionalSensitive(account.AccessTokenRef, maxSecretReferenceLength) {
			return invalidArgument("init", "credential reference is invalid")
		}
		if account.ClientID != "" || account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "client_id, app_id, and token_store are outside this static public-read adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by public Volume reads")
		}
		if account.Approval.AccountType != "" {
			return invalidArgument("init", "approval.account_type is not used")
		}
		if account.AccessTokenRef == "" {
			if len(account.Approval.Scopes) != 0 {
				return invalidArgument("init", "OAuth scopes require access_token_ref")
			}
		} else if !onlyBooksScope(account.Approval.Scopes) {
			return invalidArgument("init", "OAuth access tokens must record the Books scope")
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
	apiKey, err := resolveOptionalCredential(ctx, resolved.Secrets, account.SecretRef)
	if err != nil {
		return nil, authenticationError("client", err)
	}
	accessToken, err := resolveOptionalCredential(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil {
		return nil, authenticationError("client", err)
	}
	if apiKey == "" && accessToken == "" {
		return nil, authenticationError("client", nil)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	httpClient.Jar = nil
	return &Client{
		accountID: accountID, httpClient: &httpClient, clock: resolved.Clock,
		apiKey: apiKey, accessToken: accessToken,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

func resolveOptionalCredential(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	if reference == "" {
		return "", nil
	}
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validCredential(value) {
		return "", errInvalidCredential
	}
	return value, nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
