// Package googledatamanager implements Google Data Manager API v1 event
// ingestion. Conversion telemetry remains separate from organic social APIs.
package googledatamanager

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName       = "google/data-manager-api-v1"
	platformName      = "google-data-manager"
	productName       = "data-manager-api"
	apiVersion        = "v1"
	discoveryRevision = "20260811"
	defaultBaseURL    = "https://datamanager.googleapis.com"
	defaultAuthURL    = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL   = "https://oauth2.googleapis.com/token"
	documentationURL  = "https://developers.google.com/data-manager/api/reference/rest/v1/events/ingest"
	approvalURL       = "https://developers.google.com/data-manager/api/devguides/quickstart/set-up-access"
	dataManagerScope  = "https://www.googleapis.com/auth/datamanager"
)

// AccountSettings selects an optional managed credential. Destinations are
// request-scoped because one credential can write to multiple Google products.
type AccountSettings struct {
	RefreshTokenRef     string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
	ServiceAccountEmail string `json:"service_account_email,omitempty" yaml:"service_account_email,omitempty"`
	PrivateKeyRef       string `json:"private_key_ref,omitempty" yaml:"private_key_ref,omitempty"`
}

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
		return invalidArgument("init", "product must be data-manager-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter-level settings are not supported; Google API and OAuth endpoints are fixed")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		staticToken := account.AccessTokenRef != ""
		managedOAuth := account.ClientID != "" || account.SecretRef != "" || typed.RefreshTokenRef != ""
		serviceAccount := typed.ServiceAccountEmail != "" || typed.PrivateKeyRef != ""
		if boolInt(staticToken)+boolInt(managedOAuth)+boolInt(serviceAccount) != 1 {
			return invalidArgument("init", "configure exactly one credential mode: access_token_ref; client_id, secret_ref, and refresh_token_ref; or service_account_email and private_key_ref")
		}
		if staticToken && !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is invalid")
		}
		if managedOAuth && (!validOpaque(account.ClientID, 1024) || !validOpaque(account.SecretRef, 4096) || !validOpaque(typed.RefreshTokenRef, 4096)) {
			return invalidArgument("init", "client_id, secret_ref, and refresh_token_ref are required for user OAuth")
		}
		if serviceAccount && (!validServiceAccountEmail(typed.ServiceAccountEmail) || !validOpaque(typed.PrivateKeyRef, 4096)) {
			return invalidArgument("init", "service_account_email and private_key_ref are required for service-account authentication")
		}
		if staticToken {
			if len(account.Approval.Scopes) != 0 && !validOAuthScopes(account.Approval.Scopes) {
				return invalidArgument("init", "static-token approval scopes must be empty or exactly the Data Manager scope")
			}
		} else if !validOAuthScopes(account.Approval.Scopes) {
			return invalidArgument("init", "managed credentials require exactly the Data Manager scope")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) || account.Approval.AccountType != "" {
			return invalidArgument("init", "app_id, token_store, webhook, and approval.account_type are not supported by Data Manager event ingestion")
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
		clientScopes = []string{dataManagerScope}
		tokens = &serviceAccountTokenSource{
			client: ServiceAccountClient{Email: typed.ServiceAccountEmail, PrivateKey: privateKey, TokenURL: defaultTokenURL, HTTPClient: httpClient, Clock: resolved.Clock, Scopes: clientScopes},
			store:  resolved.TokenStore,
			key:    socialhub.TokenKey{Platform: platformName, Product: productName, Tenant: typed.ServiceAccountEmail, Account: string(accountID), Scopes: dataManagerScope},
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
		clientScopes = []string{dataManagerScope}
		tokens = &refreshTokenSource{
			oauth:        OAuthClient{ClientID: account.ClientID, ClientSecret: secret, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL, HTTPClient: httpClient, Clock: resolved.Clock, Scopes: clientScopes},
			refreshToken: refreshToken, store: resolved.TokenStore,
			key: socialhub.TokenKey{Platform: platformName, Product: productName, Tenant: account.ClientID, Account: string(accountID), Scopes: dataManagerScope},
		}
	}
	api, err := transport.New(defaultBaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock))
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api, scopes: clientScopes}, nil
}

// OAuth returns Google's user authorization-code and refresh-token helper.
func (adapter *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	options := adapter.options
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
	return &OAuthClient{ClientID: account.ClientID, ClientSecret: secret, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL, HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock, Scopes: []string{dataManagerScope}}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Google credential", err, reference, value)
	}
	if !validOpaque(value, 16384) {
		return "", authenticationError(operation, "resolved Google credential is invalid", nil, reference, value)
	}
	return value, nil
}

func resolvePrivateKey(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	value = strings.TrimSpace(value)
	if err != nil {
		return "", authenticationError("client", "could not resolve the Google service-account private key", err, reference, value)
	}
	if value == "" || len(value) > 64*1024 || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return "", authenticationError("client", "resolved Google service-account private key is invalid", nil, reference, value)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
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
