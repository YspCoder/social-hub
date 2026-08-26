// Package skimlinks implements publisher-facing Skimlinks Merchant, Link
// Wrapper, and Reporting API workflows.
package skimlinks

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName              = "skimlinks/publisher-apis-v4"
	platformName             = "skimlinks"
	productName              = "publisher-apis"
	apiVersion               = "merchant-v4"
	defaultMerchantBaseURL   = "https://merchants.skimapis.com"
	defaultReportingBaseURL  = "https://reporting.skimapis.com"
	defaultAuthenticationURL = "https://authentication.skimapis.com/access_token"
	defaultLinkBaseURL       = "https://go.skimresources.com"
	documentationURL         = "https://developers.skimlinks.com/"
)

// Settings controls the official Skimlinks endpoints. Overrides are intended
// for controlled contract-verification gateways.
type Settings struct {
	MerchantBaseURL   string `json:"merchant_base_url,omitempty" yaml:"merchant_base_url,omitempty"`
	ReportingBaseURL  string `json:"reporting_base_url,omitempty" yaml:"reporting_base_url,omitempty"`
	AuthenticationURL string `json:"authentication_url,omitempty" yaml:"authentication_url,omitempty"`
	LinkBaseURL       string `json:"link_base_url,omitempty" yaml:"link_base_url,omitempty"`
}

// AccountSettings binds one social-hub account to a Skimlinks publisher and
// registered site. PublisherDomainID selects the site-specific merchant rates;
// SiteID is the Link Wrapper's domain-specific id value.
type AccountSettings struct {
	PublisherID       int64  `json:"publisher_id" yaml:"publisher_id"`
	PublisherDomainID int64  `json:"publisher_domain_id" yaml:"publisher_domain_id"`
	SiteID            string `json:"site_id" yaml:"site_id"`
}

// Adapter implements socialhub.Adapter for the current publisher-facing suite.
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
		MerchantBaseURL: defaultMerchantBaseURL, ReportingBaseURL: defaultReportingBaseURL,
		AuthenticationURL: defaultAuthenticationURL, LinkBaseURL: defaultLinkBaseURL,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.MerchantBaseURL) || !validEndpoint(settings.ReportingBaseURL) ||
		!validEndpoint(settings.AuthenticationURL) || !validEndpoint(settings.LinkBaseURL) {
		return invalidArgument("init", "all endpoint settings must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validAccountSettings(typed) {
			return invalidArgument("init", "account settings require positive publisher_id and publisher_domain_id plus a valid site_id")
		}
		staticToken := validOpaque(account.AccessTokenRef, 4096)
		managedToken := account.AccessTokenRef == "" && validOpaque(account.ClientID, 1024) && validOpaque(account.SecretRef, 4096)
		if !staticToken && !managedToken {
			return invalidArgument("init", "configure access_token_ref or client_id and secret_ref")
		}
		if staticToken && (account.ClientID != "" || account.SecretRef != "") {
			return invalidArgument("init", "client credentials cannot be combined with access_token_ref")
		}
		if staticToken && account.TokenStore != "" {
			return invalidArgument("init", "token_store is only used with managed client credentials")
		}
		if account.TokenStore != "" && !validOpaque(account.TokenStore, 256) {
			return invalidArgument("init", "token_store is invalid")
		}
		if account.AppID != "" {
			return invalidArgument("init", "app_id is not used by Skimlinks publisher APIs")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by these request/response workflows")
		}
		if len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth scopes are not used by Skimlinks client credentials")
		}
		if account.Approval.AccountType != "" && !validOpaque(account.Approval.AccountType, 256) {
			return invalidArgument("init", "approval.account_type is invalid")
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
			AccessToken: accessToken, Scopes: []string{strconv.FormatInt(typed.PublisherID, 10)},
		}}
		configuredSecrets := []string{accessToken}
		errorSecrets = func() []string { return append([]string(nil), configuredSecrets...) }
	} else {
		clientSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		source := &clientCredentialsTokenSource{
			client: tokenClient{
				URL: settings.AuthenticationURL, ClientID: account.ClientID, ClientSecret: clientSecret,
				HTTPClient: httpClient, Clock: resolved.Clock, PublisherID: typed.PublisherID,
			},
			configuredSecrets: []string{account.ClientID, clientSecret}, store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Subject: strconv.FormatInt(typed.PublisherID, 10),
			},
		}
		tokens, errorSecrets = source, source.redactionSecrets
	}
	authenticator := transport.QueryAuthenticator("access_token")
	decodeError := newHTTPErrorDecoder(resolved.Clock, errorSecrets)
	merchantAPI, err := transport.NewWithAuthenticator(
		settings.MerchantBaseURL, httpClient, tokens, platformName, productName,
		authenticator, decodeError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	reportingAPI, err := transport.NewWithAuthenticator(
		settings.ReportingBaseURL, httpClient, tokens, platformName, productName,
		authenticator, decodeError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, publisherID: typed.PublisherID, publisherDomainID: typed.PublisherDomainID,
		siteID: typed.SiteID, linkBaseURL: settings.LinkBaseURL, merchantAPI: merchantAPI,
		reportingAPI: reportingAPI, errorSecrets: errorSecrets, approval: account.Approval,
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
