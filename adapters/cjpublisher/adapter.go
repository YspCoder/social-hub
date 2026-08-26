// Package cjpublisher implements CJ publisher GraphQL APIs and Link Search v2.
package cjpublisher

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "cj/publisher-graphql-link-search-v2"
	platformName     = "cj"
	productName      = "publisher-apis"
	apiVersion       = "GraphQL unversioned; Link Search v2"
	documentationURL = "https://developers.cj.com/"

	defaultProductsBaseURL    = "https://ads.api.cj.com"
	defaultCommissionsBaseURL = "https://commissions.api.cj.com"
	defaultProgramsBaseURL    = "https://programs.api.cj.com"
	defaultLinksBaseURL       = "https://link-search.api.cj.com"
)

// Settings controls CJ's four publisher API origins. Overrides are intended
// for a controlled contract-verification gateway.
type Settings struct {
	ProductsBaseURL    string `json:"products_base_url,omitempty" yaml:"products_base_url,omitempty"`
	CommissionsBaseURL string `json:"commissions_base_url,omitempty" yaml:"commissions_base_url,omitempty"`
	ProgramsBaseURL    string `json:"programs_base_url,omitempty" yaml:"programs_base_url,omitempty"`
	LinksBaseURL       string `json:"links_base_url,omitempty" yaml:"links_base_url,omitempty"`
}

// AccountSettings binds one social-hub account to a CJ publisher company.
type AccountSettings struct {
	PublisherID string `json:"publisher_id" yaml:"publisher_id"`
	WebsiteID   string `json:"website_id,omitempty" yaml:"website_id,omitempty"`
}

// Adapter implements socialhub.Adapter for CJ's current public publisher
// GraphQL surfaces and Link Search v2.
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
		return invalidArgument("init", "product must be publisher-apis")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{
		ProductsBaseURL: defaultProductsBaseURL, CommissionsBaseURL: defaultCommissionsBaseURL,
		ProgramsBaseURL: defaultProgramsBaseURL, LinksBaseURL: defaultLinksBaseURL,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{
		settings.ProductsBaseURL, settings.CommissionsBaseURL, settings.ProgramsBaseURL, settings.LinksBaseURL,
	} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all base URLs must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
		}
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) || account.ClientID != "" || account.SecretRef != "" || account.AppID != "" {
			return invalidArgument("init", "access_token_ref is required; client_id, secret_ref, and app_id are not used")
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
		if !validIdentifier(typed.PublisherID) || !validOptionalIdentifier(typed.WebsiteID) {
			return invalidArgument("init", "account.settings.publisher_id is required and website_id must be a valid optional PID")
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{
		AccessToken: accessToken, TokenType: "Bearer", Scopes: append([]string(nil), account.Approval.Scopes...),
	}}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	decoder := newHTTPErrorDecoder(resolved.Clock, accessToken)
	products, err := transport.New(settings.ProductsBaseURL, httpClient, tokens, platformName, productName, decoder)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	commissions, err := transport.New(settings.CommissionsBaseURL, httpClient, tokens, platformName, productName, decoder)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	programs, err := transport.New(settings.ProgramsBaseURL, httpClient, tokens, platformName, productName, decoder)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	links, err := transport.New(settings.LinksBaseURL, httpClient, tokens, platformName, productName, decoder)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, publisherID: typed.PublisherID, websiteID: typed.WebsiteID,
		productsAPI: products, commissionsAPI: commissions, programsAPI: programs, linksAPI: links,
		httpClient: httpClient, errorDecoder: decoder, approval: account.Approval,
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
