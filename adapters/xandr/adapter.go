// Package xandr implements read-only Microsoft Advertising/Xandr Digital
// Platform API advertiser and campaign workflows.
package xandr

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "xandr/digital-platform-api"
	platformName     = "xandr"
	productName      = "digital-platform-api"
	apiVersion       = "continuous"
	defaultBaseURL   = "https://api.appnexus.com"
	defaultAuthURL   = defaultBaseURL + "/auth"
	documentationURL = "https://learn.microsoft.com/en-us/xandr/digital-platform-api/"
)

// Adapter implements socialhub.Adapter for the Digital Platform API.
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
		return invalidArgument("init", "product must be digital-platform-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; the Xandr API origin is fixed")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 512) || !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "client_id and secret_ref are required for Xandr username/password authentication")
		}
		if account.AppID != "" || account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "app_id, access_token_ref, and token_store are not supported for Xandr sessions")
		}
		if len(account.Settings) != 0 {
			return invalidArgument("init", "account-specific settings are not supported")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are outside this adapter")
		}
		if len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "Digital Platform API permissions are account grants, not OAuth scopes")
		}
		if account.Approval.AccountType != "" && account.Approval.AccountType != productName {
			return invalidArgument("init", "approval.account_type must be digital-platform-api")
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
	password, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef)
	if err != nil {
		return nil, err
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	requestIDs := newRequestIDFilter(account.ClientID, password, account.SecretRef, string(account.ID))
	sessions := &sessionTokenSource{
		username: account.ClientID, password: password,
		httpClient: httpClient, clock: resolved.Clock, requestIDs: requestIDs,
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validOpaque(token.AccessToken, 16_384) {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("Authorization", token.AccessToken)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, sessions, platformName, productName,
		authenticator, newHTTPErrorDecoder(resolved.Clock, requestIDs),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api, sessions: sessions, clock: resolved.Clock}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: "client",
			PlatformMessage: "configured Xandr password could not be resolved",
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
