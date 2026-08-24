// Package youtubereporting implements account-bound YouTube Reporting API v1
// bulk-report job, metadata, and secure CSV download workflows.
package youtubereporting

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
	adapterName       = "google/youtube-reporting-api-v1"
	platformName      = "youtube"
	productName       = "youtube-reporting-api"
	apiVersion        = "v1"
	discoveryRevision = "20260805"
	defaultBaseURL    = "https://youtubereporting.googleapis.com"
	defaultAuthURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL   = "https://oauth2.googleapis.com/token"
	documentationURL  = "https://developers.google.com/youtube/reporting/v1/reference/rest"

	analyticsReadScope    = "https://www.googleapis.com/auth/yt-analytics.readonly"
	analyticsRevenueScope = "https://www.googleapis.com/auth/yt-analytics-monetary.readonly"
)

type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL  string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings binds an account to the current OAuth user's channel when
// ContentOwnerID is empty, or to one YouTube CMS content owner otherwise.
type AccountSettings struct {
	ContentOwnerID      string `json:"content_owner_id,omitempty" yaml:"content_owner_id,omitempty"`
	RefreshTokenRef     string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
	ServiceAccountEmail string `json:"service_account_email,omitempty" yaml:"service_account_email,omitempty"`
	PrivateKeyRef       string `json:"private_key_ref,omitempty" yaml:"private_key_ref,omitempty"`
}

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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be youtube-reporting-api")
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
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.AuthURL) || !validEndpoint(settings.TokenURL) {
		return invalidArgument("init", "Google endpoints must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.ContentOwnerID != "" && !validIdentifier(typed.ContentOwnerID, 256) {
			return invalidArgument("init", "account settings.content_owner_id is invalid")
		}
		staticToken := strings.TrimSpace(account.AccessTokenRef) != ""
		managedOAuth := !staticToken && validOpaque(account.ClientID, 1024) &&
			strings.TrimSpace(account.SecretRef) != "" && strings.TrimSpace(typed.RefreshTokenRef) != ""
		serviceAccount := !staticToken && validServiceAccountEmail(typed.ServiceAccountEmail) &&
			strings.TrimSpace(typed.PrivateKeyRef) != ""
		if boolInt(staticToken)+boolInt(managedOAuth)+boolInt(serviceAccount) != 1 {
			return invalidArgument("init", "configure exactly one credential mode: access_token_ref; client_id, secret_ref, and refresh_token_ref; or content_owner_id, service_account_email, and private_key_ref")
		}
		if staticToken {
			if typed.RefreshTokenRef != "" || typed.ServiceAccountEmail != "" || typed.PrivateKeyRef != "" {
				return invalidArgument("init", "access_token_ref cannot be combined with managed credential settings")
			}
			if (account.ClientID == "") != (account.SecretRef == "") {
				return invalidArgument("init", "client_id and secret_ref must be configured together")
			}
		}
		if managedOAuth && (typed.ServiceAccountEmail != "" || typed.PrivateKeyRef != "") {
			return invalidArgument("init", "user OAuth cannot be combined with service-account settings")
		}
		if serviceAccount {
			if typed.ContentOwnerID == "" {
				return invalidArgument("init", "service-account authentication requires content_owner_id")
			}
			if account.ClientID != "" || account.SecretRef != "" || typed.RefreshTokenRef != "" {
				return invalidArgument("init", "service-account authentication cannot be combined with user OAuth settings")
			}
		}
		if len(account.Approval.Scopes) > 0 && !validOAuthScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval scopes must contain unique YouTube Reporting scopes")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by YouTube Reporting API")
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
	clientScopes := append([]string(nil), account.Approval.Scopes...)
	var tokens socialhub.TokenSource
	switch {
	case strings.TrimSpace(account.AccessTokenRef) != "":
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer", Scopes: clientScopes}}
	case typed.ServiceAccountEmail != "":
		privateKeyPEM, err := resolvePrivateKey(ctx, resolved.Secrets, typed.PrivateKeyRef)
		if err != nil {
			return nil, err
		}
		privateKey, err := parseRSAPrivateKey(privateKeyPEM)
		if err != nil {
			return nil, invalidArgument("client", "service-account private key is not a valid RSA PKCS#8 or PKCS#1 PEM key")
		}
		clientScopes = oauthScopes(account.Approval.Scopes)
		tokens = &serviceAccountTokenSource{
			client: ServiceAccountClient{Email: typed.ServiceAccountEmail, PrivateKey: privateKey, TokenURL: settings.TokenURL, HTTPClient: httpClient, Clock: resolved.Clock, Scopes: clientScopes},
			store:  resolved.TokenStore,
			key:    socialhub.TokenKey{Platform: platformName, Product: productName, Tenant: typed.ServiceAccountEmail, Account: string(accountID), Scopes: strings.Join(clientScopes, " ")},
		}
	default:
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		refreshToken, err := resolveSecret(ctx, resolved.Secrets, typed.RefreshTokenRef, "client")
		if err != nil {
			return nil, err
		}
		clientScopes = oauthScopes(account.Approval.Scopes)
		tokens = &refreshTokenSource{
			oauth:        OAuthClient{ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL, HTTPClient: httpClient, Clock: resolved.Clock, Scopes: clientScopes},
			refreshToken: refreshToken, store: resolved.TokenStore,
			key: socialhub.TokenKey{Platform: platformName, Product: productName, Tenant: account.ClientID, Account: string(accountID), Scopes: strings.Join(clientScopes, " ")},
		}
	}
	api, err := transport.New(settings.BaseURL, httpClient, tokens, platformName, productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	return &Client{accountID: accountID, contentOwnerID: typed.ContentOwnerID, api: api, httpClient: httpClient, baseURL: baseURL, scopes: clientScopes}, nil
}

// OAuth returns Google's user authorization-code and refresh-token helper.
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
		return nil, invalidArgument("oauth", "client_id and secret_ref are required for user OAuth")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL, HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock, Scopes: oauthScopes(account.Approval.Scopes)}, nil
}

func oauthScopes(configured []string) []string {
	if len(configured) == 0 {
		return []string{analyticsReadScope}
	}
	result := make([]string, 0, len(supportedScopes))
	for _, candidate := range supportedScopes {
		for _, scope := range configured {
			if scope == candidate {
				result = append(result, candidate)
				break
			}
		}
	}
	return result
}

var supportedScopes = []string{analyticsReadScope, analyticsRevenueScope}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16384) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func resolvePrivateKey(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	value = strings.TrimSpace(value)
	if err != nil || value == "" || len(value) > 64*1024 || strings.ContainsRune(value, '\x00') {
		return "", platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
