// Package microsoftads implements account-scoped Microsoft Advertising REST
// API v13 workflows. Paid advertising resources remain separate from common
// social posts.
package microsoftads

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName             = "microsoftads/api-v13"
	platformName            = "microsoftads"
	productName             = "api"
	apiVersion              = "v13"
	defaultCampaignBaseURL  = "https://campaign.api.bingads.microsoft.com/CampaignManagement/v13"
	defaultCustomerBaseURL  = "https://clientcenter.api.bingads.microsoft.com/CustomerManagement/v13"
	defaultReportingBaseURL = "https://reporting.api.bingads.microsoft.com/Reporting/v13"
	defaultAuthURL          = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	defaultTokenURL         = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	documentationURL        = "https://learn.microsoft.com/en-us/advertising/guides/?view=bingads-13"
	adsManageScope          = "https://ads.microsoft.com/msads.manage"
	defaultMaxReportBytes   = int64(256 << 20)
	maximumReportBytes      = int64(1 << 30)
)

// Settings controls Microsoft Advertising REST, OAuth, and report download
// limits. Endpoint overrides are intended for tests and private HTTP gateways.
type Settings struct {
	CampaignBaseURL  string `json:"campaign_base_url,omitempty" yaml:"campaign_base_url,omitempty"`
	CustomerBaseURL  string `json:"customer_base_url,omitempty" yaml:"customer_base_url,omitempty"`
	ReportingBaseURL string `json:"reporting_base_url,omitempty" yaml:"reporting_base_url,omitempty"`
	AuthURL          string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL         string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	MaxReportBytes   int64  `json:"max_report_bytes,omitempty" yaml:"max_report_bytes,omitempty"`
}

// AccountSettings binds one social-hub account to one Microsoft Advertising
// manager customer and ad account.
type AccountSettings struct {
	CustomerID        string `json:"customer_id" yaml:"customer_id"`
	CustomerAccountID string `json:"customer_account_id" yaml:"customer_account_id"`
	DeveloperTokenRef string `json:"developer_token_ref" yaml:"developer_token_ref"`
}

// Adapter implements socialhub.Adapter for Microsoft Advertising REST API v13.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC),
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
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{
		CampaignBaseURL: defaultCampaignBaseURL, CustomerBaseURL: defaultCustomerBaseURL,
		ReportingBaseURL: defaultReportingBaseURL, AuthURL: defaultAuthURL,
		TokenURL: defaultTokenURL, MaxReportBytes: defaultMaxReportBytes,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{
		settings.CampaignBaseURL, settings.CustomerBaseURL, settings.ReportingBaseURL,
		settings.AuthURL, settings.TokenURL,
	} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "Microsoft endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	if settings.MaxReportBytes <= 0 || settings.MaxReportBytes > maximumReportBytes {
		return invalidArgument("init", "max_report_bytes must be between 1 and 1073741824")
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every Microsoft Advertising account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validNumericID(typed.CustomerID) {
			return invalidArgument("init", "account.settings.customer_id must be a nonzero numeric ID")
		}
		if !validNumericID(typed.CustomerAccountID) {
			return invalidArgument("init", "account.settings.customer_account_id must be a nonzero numeric ID")
		}
		if strings.TrimSpace(typed.DeveloperTokenRef) == "" {
			return invalidArgument("init", "account.settings.developer_token_ref is required")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by this adapter version")
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	developerToken, err := resolveSecret(ctx, resolved.Secrets, typed.DeveloperTokenRef, "client")
	if err != nil {
		return nil, err
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	fullAuthenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set("DeveloperToken", developerToken)
		request.Header.Set("CustomerId", typed.CustomerID)
		request.Header.Set("CustomerAccountId", typed.CustomerAccountID)
		return nil
	})
	customerAuthenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set("DeveloperToken", developerToken)
		return nil
	})
	campaign, err := transport.NewWithAuthenticator(
		settings.CampaignBaseURL, &httpClient, tokens, platformName, productName, fullAuthenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	customer, err := transport.NewWithAuthenticator(
		settings.CustomerBaseURL, &httpClient, tokens, platformName, productName, customerAuthenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	reporting, err := transport.NewWithAuthenticator(
		settings.ReportingBaseURL, &httpClient, tokens, platformName, productName, fullAuthenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	reportingBase, _ := url.Parse(settings.ReportingBaseURL)
	return &Client{
		accountID: accountID, customerID: typed.CustomerID, customerAccountID: typed.CustomerAccountID,
		campaign: campaign, customer: customer, reporting: reporting, httpClient: &httpClient,
		reportingBaseURL: reportingBase, maxReportBytes: settings.MaxReportBytes,
		scopes: append([]string(nil), account.Approval.Scopes...),
	}, nil
}

// OAuth returns Microsoft's authorization-code and refresh-token helper.
func (adapter *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	settings, options := adapter.settings, adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if !validOpaque(account.ClientID, 1024) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: settings.TokenURL, HTTPClient: &httpClient, Clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 8192) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
