// Package ads implements account-scoped X Ads API v12 workflows. Paid-media
// resources remain separate from the organic x/v2 adapter.
package ads

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dghubble/oauth1"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName            = "x/ads-api-v12"
	platformName           = "x"
	productName            = "ads-api"
	apiVersion             = "v12"
	defaultBaseURL         = "https://ads-api.x.com/12"
	defaultRequestTokenURL = "https://api.x.com/oauth/request_token"
	defaultAuthorizeURL    = "https://api.x.com/oauth/authorize"
	defaultAccessTokenURL  = "https://api.x.com/oauth/access_token"
	documentationURL       = "https://docs.x.com/x-ads-api"
)

// Settings controls X Ads API and OAuth endpoints. Overrides are intended for
// tests and controlled gateways.
type Settings struct {
	BaseURL         string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	RequestTokenURL string `json:"request_token_url,omitempty" yaml:"request_token_url,omitempty"`
	AuthorizeURL    string `json:"authorize_url,omitempty" yaml:"authorize_url,omitempty"`
	AccessTokenURL  string `json:"access_token_url,omitempty" yaml:"access_token_url,omitempty"`
}

// AccountSettings binds one social-hub account to one X Ads Account and its
// OAuth 1.0a access-token secret.
type AccountSettings struct {
	AdsAccountID         string `json:"ads_account_id" yaml:"ads_account_id"`
	AccessTokenSecretRef string `json:"access_token_secret_ref" yaml:"access_token_secret_ref"`
}

// Adapter implements socialhub.Adapter for X Ads API v12.
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
		DocURL:     documentationURL + "/fundamentals/versioning",
		VerifiedAt: time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC),
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
	settings := Settings{
		BaseURL: defaultBaseURL, RequestTokenURL: defaultRequestTokenURL,
		AuthorizeURL: defaultAuthorizeURL, AccessTokenURL: defaultAccessTokenURL,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.RequestTokenURL, settings.AuthorizeURL, settings.AccessTokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "X endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.ClientID, 1024) || strings.TrimSpace(account.SecretRef) == "" || strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "client_id, secret_ref, and access_token_ref are required for every X Ads account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validAdsID(typed.AdsAccountID) || strings.TrimSpace(typed.AccessTokenSecretRef) == "" {
			return invalidArgument("init", "account.settings requires a base36 ads_account_id and access_token_secret_ref")
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
	consumerSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	accessTokenSecret, err := resolveSecret(ctx, resolved.Secrets, typed.AccessTokenSecretRef, "client")
	if err != nil {
		return nil, err
	}
	oauthConfig := &oauth1.Config{ConsumerKey: account.ClientID, ConsumerSecret: consumerSecret}
	signed := signedHTTPClient(resolved.HTTPClient, oauthConfig, oauth1.NewToken(accessToken, accessTokenSecret))
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "oauth1"}}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, signed, tokens, platformName, productName,
		transport.AuthenticatorFunc(func(*http.Request, socialhub.Token) error { return nil }),
		newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, adsAccountID: typed.AdsAccountID, api: api,
		accessLevel: strings.TrimSpace(account.Approval.AccountType),
	}, nil
}

// OAuth returns X's three-legged OAuth 1.0a helper. Ads API access is granted
// separately through X Ads API Standard Access and account roles.
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
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	return &OAuthClient{
		ConsumerKey: account.ClientID, ConsumerSecret: secret,
		RequestTokenURL: settings.RequestTokenURL, AuthorizeURL: settings.AuthorizeURL,
		AccessTokenURL: settings.AccessTokenURL, HTTPClient: &httpClient,
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
