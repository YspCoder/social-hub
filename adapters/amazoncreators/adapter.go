// Package amazoncreators implements the Amazon Creators API Catalog v1
// affiliate product-discovery workflows.
package amazoncreators

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "amazon/creators-api-catalog-v1"
	platformName     = "amazon"
	productName      = "creators-api-catalog"
	apiVersion       = "v1"
	sdkVersion       = "1.2.0"
	defaultBaseURL   = "https://creatorsapi.amazon"
	documentationURL = "https://affiliate-program.amazon.com/creatorsapi/docs/en-us/introduction"
	oauthScope       = "creatorsapi::default"
	associatesType   = "approved-amazon-associates-store"
)

// AccountSettings contains the store, marketplace, and OAuth-region binding.
type AccountSettings struct {
	Marketplace       string `json:"marketplace" yaml:"marketplace"`
	PartnerTag        string `json:"partner_tag" yaml:"partner_tag"`
	CredentialVersion string `json:"credential_version,omitempty" yaml:"credential_version,omitempty"`
}

// Adapter implements socialhub.Adapter for Amazon Creators API Catalog v1.
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
		Name: adapterName, Product: productName, APIVersion: apiVersion, SDKVersion: sdkVersion,
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
		return invalidArgument("init", "product must be creators-api-catalog")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, account := range config.Accounts {
		staticToken := strings.TrimSpace(account.AccessTokenRef) != ""
		managedOAuth := validOpaque(account.ClientID, 1024) && strings.TrimSpace(account.SecretRef) != ""
		if staticToken == managedOAuth {
			return invalidArgument("init", "configure exactly one of access_token_ref or client_id with secret_ref")
		}
		if staticToken {
			if !validOpaque(account.AccessTokenRef, 4096) || account.ClientID != "" || account.SecretRef != "" {
				return invalidArgument("init", "access_token_ref cannot be combined with client_id or secret_ref")
			}
		} else if !validOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref for the Creators API credential secret is required")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "app_id, token_store, and webhook settings are not used by this adapter")
		}
		if account.Approval.AccountType != "" && account.Approval.AccountType != associatesType {
			return invalidArgument("init", "approval.account_type may only be approved-amazon-associates-store")
		}
		for _, scope := range account.Approval.Scopes {
			if scope != oauthScope {
				return invalidArgument("init", "Creators API v3 supports only the creatorsapi::default scope")
			}
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validMarketplace(typed.Marketplace) || !validOpaque(typed.PartnerTag, 64) {
			return invalidArgument("init", "account.settings.marketplace and partner_tag are required and must be valid")
		}
		if staticToken {
			if typed.CredentialVersion != "" {
				return invalidArgument("init", "credential_version is only valid for managed OAuth accounts")
			}
		} else if !validCredentialVersion(typed.CredentialVersion) {
			return invalidArgument("init", "managed OAuth requires credential_version 3.1, 3.2, or 3.3")
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
	var tokens socialhub.TokenSource
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{
			AccessToken: accessToken, TokenType: "Bearer", Scopes: []string{oauthScope},
		}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &applicationTokenSource{
			oauth: OAuthClient{
				CredentialID: account.ClientID, CredentialSecret: secret, CredentialVersion: typed.CredentialVersion,
				HTTPClient: httpClient, Clock: resolved.Clock,
			},
			store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Scopes: oauthScope,
			},
		}
	}
	api, err := transport.New(defaultBaseURL, httpClient, tokens, platformName, productName, newHTTPErrorDecoder(resolved.Clock))
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, api: api, approval: account.Approval,
		marketplace: typed.Marketplace, partnerTag: typed.PartnerTag,
	}, nil
}

// OAuth returns the Client Credentials helper for a managed OAuth account.
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
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		return nil, invalidArgument("oauth", "managed OAuth is unavailable for a static-token account")
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		CredentialID: account.ClientID, CredentialSecret: secret, CredentialVersion: typed.CredentialVersion,
		HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Amazon credential", err, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved Amazon credential is invalid", nil, value)
	}
	return value, nil
}

func cloneHTTPClient(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	copy.Jar = nil
	return &copy
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
