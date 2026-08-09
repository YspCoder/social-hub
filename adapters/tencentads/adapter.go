// Package tencentads implements Tencent Ads Marketing API v1.3 for one
// advertiser account. Paid advertising resources remain behind typed
// workflows and are not mapped onto organic social posts.
package tencentads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "tencentads/marketing-api-v1.3"
	platformName     = "tencentads"
	productName      = "marketing-api"
	apiVersion       = "v1.3"
	defaultBaseURL   = "https://api.e.qq.com/v1.3"
	defaultAuthURL   = "https://developers.e.qq.com"
	defaultTokenURL  = "https://api.e.qq.com"
	documentationURL = "https://s.apifox.cn/apidoc/docs-site/3515798"
)

// Settings controls API and OAuth origins. Overrides are intended for tests
// and private HTTP gateways.
type Settings struct {
	BaseURL              string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthorizationBaseURL string `json:"authorization_base_url,omitempty" yaml:"authorization_base_url,omitempty"`
	TokenBaseURL         string `json:"token_base_url,omitempty" yaml:"token_base_url,omitempty"`
}

// AccountSettings binds a social-hub account to one Tencent Ads advertiser.
type AccountSettings struct {
	AccountID int64 `json:"account_id" yaml:"account_id"`
}

// Adapter implements socialhub.Adapter for Tencent Ads Marketing API v1.3.
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
		return invalidArgument("init", "product must be marketing-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{
		BaseURL: defaultBaseURL, AuthorizationBaseURL: defaultAuthURL, TokenBaseURL: defaultTokenURL,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validEndpoint(settings.AuthorizationBaseURL) || !validEndpoint(settings.TokenBaseURL) {
		return invalidArgument("init", "API and OAuth base URLs must be absolute HTTP(S) URLs without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every advertiser account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validID(typed.AccountID) {
			return invalidArgument("init", "account.settings.account_id must be a positive integer")
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		nonce, err := randomNonce()
		if err != nil {
			return err
		}
		query := request.URL.Query()
		query.Set("access_token", token.AccessToken)
		query.Set("timestamp", strconv.FormatInt(resolved.Clock.Now().Unix(), 10))
		query.Set("nonce", nonce)
		request.URL.RawQuery = query.Encode()
		return nil
	})
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "TencentAds"}}
	api, err := transport.NewWithAuthenticator(
		settings.BaseURL, &httpClient, tokens, platformName, productName,
		authenticator, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, advertiserID: typed.AccountID, api: api}, nil
}

// OAuth returns helpers for authorization-code and refresh-token flows.
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
	clientID, err := strconv.ParseInt(account.AppID, 10, 64)
	if err != nil || !validID(clientID) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "a numeric app_id and secret_ref are required")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	return &OAuthClient{
		ClientID: clientID, ClientSecret: secret,
		AuthorizationBaseURL: settings.AuthorizationBaseURL, TokenBaseURL: settings.TokenBaseURL,
		HTTPClient: &httpClient, Clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 8192) {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func randomNonce() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
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
