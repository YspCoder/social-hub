// Package yahoosearchads implements LINE Yahoo Ads Search Ads API v20
// management and reporting workflows.
package yahoosearchads

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "line-yahoo/search-ads-api-v20"
	platformName     = "line-yahoo"
	productName      = "search-ads-api"
	apiVersion       = "v20"
	defaultBaseURL   = "https://ads-search.yahooapis.jp/api/v20"
	defaultAuthURL   = "https://biz-oauth.yahoo.co.jp/oauth/v1/authorize"
	defaultTokenURL  = "https://biz-oauth.yahoo.co.jp/oauth/v1/token"
	documentationURL = "https://ads-developers.yahoo.co.jp/reference/ads-search-api/v20/"
	oauthScope       = "yahooads"
)

// AccountSettings binds one social-hub account to one Search Ads account.
// BaseAccountID may be an MCC account and defaults to AccountID.
type AccountSettings struct {
	AccountID       int64  `json:"account_id" yaml:"account_id"`
	BaseAccountID   int64  `json:"base_account_id,omitempty" yaml:"base_account_id,omitempty"`
	RefreshTokenRef string `json:"refresh_token_ref,omitempty" yaml:"refresh_token_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for LINE Yahoo Ads Search Ads API v20.
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
		return invalidArgument("init", "product must be search-ads-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; LINE Yahoo API and OAuth origins are fixed")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.AccountID <= 0 || typed.BaseAccountID < 0 {
			return invalidArgument("init", "account.settings.account_id must be positive and base_account_id must not be negative")
		}
		staticToken := account.AccessTokenRef != ""
		if staticToken {
			if !validOpaque(account.AccessTokenRef, 4_096) {
				return invalidArgument("init", "access_token_ref must be a valid credential reference")
			}
			if typed.RefreshTokenRef != "" {
				return invalidArgument("init", "refresh_token_ref cannot be combined with access_token_ref")
			}
			if (account.ClientID == "") != (account.SecretRef == "") {
				return invalidArgument("init", "client_id and secret_ref must be configured together")
			}
			if account.ClientID != "" && (!validOpaque(account.ClientID, 1_024) || !validOpaque(account.SecretRef, 4_096)) {
				return invalidArgument("init", "client_id or secret_ref is invalid")
			}
		} else if !validOpaque(account.ClientID, 1_024) || !validOpaque(account.SecretRef, 4_096) ||
			!validOpaque(typed.RefreshTokenRef, 4_096) {
			return invalidArgument("init", "configure valid client_id, secret_ref, and account.settings.refresh_token_ref")
		}
		if account.AppID != "" || account.TokenStore != "" || account.Approval.AccountType != "" {
			return invalidArgument("init", "app_id, token_store, and approval.account_type are not used by this adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Search Ads API")
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
	baseAccountID := typed.BaseAccountID
	if baseAccountID == 0 {
		baseAccountID = typed.AccountID
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	clientScopes := append([]string(nil), account.Approval.Scopes...)
	requestIDs := newRequestIDFilter(
		string(accountID), account.ClientID, formatID(typed.AccountID), formatID(baseAccountID),
	)
	var tokens socialhub.TokenSource
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		requestIDs.add(accessToken)
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer", Scopes: clientScopes}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		refreshToken, err := resolveSecret(ctx, resolved.Secrets, typed.RefreshTokenRef, "client")
		if err != nil {
			return nil, err
		}
		requestIDs.add(secret, refreshToken)
		tokens = &refreshTokenSource{
			oauth: OAuthClient{
				clientID: account.ClientID, clientSecret: secret,
				httpClient: httpClient, clock: resolved.Clock, requestIDs: requestIDs,
			},
			refreshToken: refreshToken, store: resolved.TokenStore,
			key: socialhub.TokenKey{
				Platform: platformName, Product: productName, Tenant: account.ClientID,
				Account: string(accountID), Scopes: oauthScope,
			},
		}
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		request.Header.Set("x-z-base-account-id", formatID(baseAccountID))
		return nil
	})
	decoder := newHTTPErrorDecoder(resolved.Clock, requestIDs)
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, tokens, platformName, productName, authenticator, decoder,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, advertiserAccountID: typed.AccountID,
		api: api, httpClient: httpClient, scopes: clientScopes,
		clock: resolved.Clock, requestIDs: requestIDs, decodeError: decoder,
	}, nil
}

// OAuth returns the official authorization-code and refresh-token helper.
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
	if !validOpaque(account.ClientID, 1_024) || !validOpaque(account.SecretRef, 4_096) {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseAccountID := typed.BaseAccountID
	if baseAccountID == 0 {
		baseAccountID = typed.AccountID
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		clientID: account.ClientID, clientSecret: secret,
		httpClient: cloneHTTPClient(options.HTTPClient), clock: options.Clock,
		requestIDs: newRequestIDFilter(
			string(accountID), account.ClientID, secret, formatID(typed.AccountID), formatID(baseAccountID),
		),
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			PlatformMessage: "configured LINE Yahoo credential could not be resolved", ApprovalURL: documentationURL,
		}
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
