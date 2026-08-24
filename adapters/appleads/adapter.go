// Package appleads implements organization-scoped Apple Ads Campaign Management API 5 workflows.
// Paid-media resources remain separate from social-hub's organic interfaces.
package appleads

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "appleads/campaign-management-api-v5"
	platformName     = "appleads"
	productName      = "campaign-management-api"
	apiVersion       = "v5"
	defaultBaseURL   = "https://api.searchads.apple.com/api/v5"
	defaultTokenURL  = "https://appleid.apple.com/auth/oauth2/token"
	documentationURL = "https://developer.apple.com/documentation/apple_ads"
	oauthScope       = "searchadsorg"
)

// Settings controls Apple Ads API and OAuth endpoints. Overrides are intended
// for tests and controlled gateways.
type Settings struct {
	BaseURL  string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
}

// AccountSettings binds one social-hub account to one Apple Ads organization.
// For managed OAuth, ClientID is AccountConfig.ClientID and private_key_ref is
// resolved as an ES256 PEM private key.
type AccountSettings struct {
	OrgID         int64  `json:"org_id" yaml:"org_id"`
	TeamID        string `json:"team_id,omitempty" yaml:"team_id,omitempty"`
	KeyID         string `json:"key_id,omitempty" yaml:"key_id,omitempty"`
	PrivateKeyRef string `json:"private_key_ref,omitempty" yaml:"private_key_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Apple Ads Campaign Management API 5.
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
		return invalidArgument("init", "product must be campaign-management-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL, TokenURL: defaultTokenURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.TokenURL) {
		return invalidArgument("init", "Apple Ads endpoints must be absolute HTTP(S) URLs without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validID(typed.OrgID) {
			return invalidArgument("init", "account.settings.org_id must be a positive integer")
		}
		staticToken := strings.TrimSpace(account.AccessTokenRef) != ""
		managedOAuth := validOpaque(account.ClientID, 1024) && validOpaque(typed.TeamID, 1024) &&
			validOpaque(typed.KeyID, 1024) && strings.TrimSpace(typed.PrivateKeyRef) != ""
		if staticToken == managedOAuth {
			return invalidArgument("init", "configure exactly one of access_token_ref or client_id with team_id, key_id, and private_key_ref")
		}
		if staticToken && (account.ClientID != "" || typed.TeamID != "" || typed.KeyID != "" || typed.PrivateKeyRef != "") {
			return invalidArgument("init", "managed OAuth fields cannot be combined with access_token_ref")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Apple Ads Campaign Management API")
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
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer", Scopes: []string{oauthScope}}}
	} else {
		privateKeyPEM, err := resolveSecret(ctx, resolved.Secrets, typed.PrivateKeyRef, "client")
		if err != nil {
			return nil, err
		}
		privateKey, err := parsePrivateKey([]byte(privateKeyPEM))
		if err != nil {
			return nil, invalidArgument("client", "resolved private key must be a P-256 EC private key in PKCS#8 or SEC1 PEM form")
		}
		oauth := OAuthClient{
			ClientID: account.ClientID, TeamID: typed.TeamID, KeyID: typed.KeyID,
			PrivateKey: privateKey, TokenURL: settings.TokenURL, HTTPClient: httpClient, Clock: resolved.Clock,
		}
		tokens = &clientTokenSource{
			oauth: oauth, store: resolved.TokenStore,
			key: socialhub.TokenKey{Platform: platformName, Product: productName, Tenant: typed.TeamID, Account: string(accountID), Subject: account.ClientID, Scopes: oauthScope},
		}
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validOpaque(token.AccessToken, 16384) {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set("X-AP-Context", "orgId="+formatID(typed.OrgID))
		return nil
	})
	api, err := transport.NewWithAuthenticator(settings.BaseURL, httpClient, tokens, platformName, productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, orgID: typed.OrgID, api: api}, nil
}

// OAuth returns the client-credentials helper for a managed OAuth account.
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		return nil, invalidArgument("oauth", "managed OAuth is unavailable for a static-token account")
	}
	privateKeyPEM, err := resolveSecret(ctx, options.Secrets, typed.PrivateKeyRef, "oauth")
	if err != nil {
		return nil, err
	}
	privateKey, err := parsePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return nil, invalidArgument("oauth", "resolved private key must be a P-256 EC private key in PKCS#8 or SEC1 PEM form")
	}
	return &OAuthClient{
		ClientID: account.ClientID, TeamID: typed.TeamID, KeyID: typed.KeyID, PrivateKey: privateKey,
		TokenURL: settings.TokenURL, HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" || len(value) > 64<<10 {
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

func formatID(value int64) string { return strconv.FormatInt(value, 10) }

var _ socialhub.Adapter = (*Adapter)(nil)
