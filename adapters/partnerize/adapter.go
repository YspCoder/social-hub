// Package partnerize implements a bounded Partnerize Partners API surface.
package partnerize

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "partnerize/partners-api-v1.4.99"
	platformName     = "partnerize"
	productName      = "partners-api"
	apiVersion       = "OpenAPI 1.4.99 (v1-v3 endpoints)"
	defaultBaseURL   = "https://api.partnerize.com"
	documentationURL = "https://api-docs.partnerize.com/partner/"
)

// Settings controls the Partnerize API origin. The override is intended for a
// controlled contract-verification gateway.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings binds one social-hub account to a Partnerize partner.
type AccountSettings struct {
	PublisherID string `json:"publisher_id" yaml:"publisher_id"`
}

// Adapter implements socialhub.Adapter for the Partnerize Partners API.
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
		return invalidArgument("init", "product must be partners-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an absolute HTTP(S) URL without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		if !validBasicUsername(account.ClientID) || !validOpaque(account.SecretRef, 4096) || account.AccessTokenRef != "" || account.AppID != "" {
			return invalidArgument("init", "client_id and secret_ref are required; access_token_ref and app_id are not used")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by these request/response workflows")
		}
		for _, scope := range account.Approval.Scopes {
			if !validOpaque(scope, 1024) {
				return invalidArgument("init", "approval scopes contain an invalid value")
			}
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validPathSegment(typed.PublisherID, 256) {
			return invalidArgument("init", "account.settings.publisher_id is required and must be a safe path segment")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	userAPIKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: userAPIKey, TokenType: "Basic"}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validBasicUsername(account.ClientID) || !validOpaque(token.AccessToken, 16_384) {
			return socialhub.ErrUnauthenticated
		}
		request.SetBasicAuth(account.ClientID, token.AccessToken)
		return nil
	})
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, httpClient, tokens, platformName, productName, authenticator,
		newHTTPErrorDecoder(resolved.Clock, account.ClientID, userAPIKey),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, publisherID: typed.PublisherID, api: api,
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
