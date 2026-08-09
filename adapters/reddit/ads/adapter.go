// Package ads implements ad-account-scoped Reddit Ads API v3 workflows.
// Paid-media resources remain separate from Reddit's organic Data API.
package ads

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
	adapterName      = "reddit/ads-api-v3"
	platformName     = "reddit"
	productName      = "ads-api"
	apiVersion       = "v3"
	defaultBaseURL   = "https://ads-api.reddit.com/api/v3"
	defaultAuthURL   = "https://www.reddit.com/api/v1/authorize"
	defaultTokenURL  = "https://www.reddit.com/api/v1/access_token"
	documentationURL = "https://ads-api.reddit.com/docs/v3/"
	readScope        = "adsread"
	editScope        = "adsedit"
)

// Settings controls Reddit API and OAuth endpoints and the mandatory
// identifiable User-Agent. Endpoint overrides are intended for tests and
// controlled gateways.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL   string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL  string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	UserAgent string `json:"user_agent" yaml:"user_agent"`
}

// AccountSettings binds one social-hub account to one Reddit Ad Account.
type AccountSettings struct {
	AdAccountID string `json:"ad_account_id" yaml:"ad_account_id"`
}

// Adapter implements socialhub.Adapter for Reddit Ads API v3.
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
		return invalidArgument("init", "product must be ads-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "Reddit endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	if !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "settings.user_agent must identify platform, app, version, and a /u/ contact")
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every Reddit Ads account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validAdAccountID(typed.AdAccountID) {
			return invalidArgument("init", "account.settings.ad_account_id must start with t2_ or a2_")
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
	httpClient := withUserAgent(resolved.HTTPClient, settings.UserAgent)
	httpClient.CheckRedirect = rejectRedirect
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	api, err := transport.New(settings.BaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock))
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	return &Client{
		accountID: accountID, adAccountID: typed.AdAccountID, api: api, baseURL: baseURL,
		scopes: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
	}, nil
}

// OAuth returns Reddit's authorization-code and refresh-token helper.
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
	httpClient := withUserAgent(options.HTTPClient, settings.UserAgent)
	httpClient.CheckRedirect = rejectRedirect
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL,
		TokenURL: settings.TokenURL, UserAgent: settings.UserAgent, HTTPClient: httpClient, Clock: options.Clock,
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

type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (transport userAgentTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("User-Agent", transport.userAgent)
	return transport.base.RoundTrip(clone)
}

func withUserAgent(client *http.Client, userAgent string) *http.Client {
	copy := *client
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	copy.Transport = userAgentTransport{base: base, userAgent: userAgent}
	return &copy
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
