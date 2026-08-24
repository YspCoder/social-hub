// Package applovinmax implements the AppLovin MAX Reporting APIs.
package applovinmax

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
	adapterName      = "applovin/max-reporting-apis"
	platformName     = "applovin"
	productName      = "max-reporting-apis"
	apiVersion       = "unversioned"
	contractVersion  = "verified-2026-08-25"
	defaultBaseURL   = "https://r.applovin.com"
	downloadOrigin   = "https://applovin-externalreports.s3.amazonaws.com"
	documentationURL = "https://support.applovin.com/en/max/max-dashboard/reports/reporting-apis"
)

// Adapter implements socialhub.Adapter for AppLovin MAX Reporting APIs.
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
		return invalidArgument("init", "product must be max-reporting-apis")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter-level settings are not supported; AppLovin API and report-download origins are fixed")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.SecretRef, 4096) || account.ClientID != "" || account.AppID != "" ||
			account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "configure only secret_ref with the AppLovin Report Key")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) || account.Approval.AccountType != "" || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "webhook and approval settings are not supported by MAX Reporting APIs")
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
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	reportKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: reportKey}}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, tokens, platformName, productName,
		transport.QueryAuthenticator("api_key"), newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	origins := map[string]struct{}{downloadOrigin: {}}
	return &Client{
		accountID: accountID, api: api, httpClient: httpClient,
		clock: resolved.Clock, downloadOrigins: origins,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the AppLovin Report Key")
	}
	if !validOpaque(value, 16384) {
		return "", authenticationError(operation, "resolved AppLovin Report Key is invalid")
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	return &copy
}

func normalizedOrigin(value string) string {
	parsed, _ := url.Parse(value)
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
