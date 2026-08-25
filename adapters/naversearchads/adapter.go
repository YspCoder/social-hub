// Package naversearchads implements NAVER Search AD API v2 management and
// reporting workflows.
package naversearchads

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	adapterName      = "naver/search-ad-api-v2"
	platformName     = "naver"
	productName      = "search-ad-api"
	apiVersion       = "management-v2/reporting-v1"
	defaultBaseURL   = "https://api.searchad.naver.com"
	documentationURL = "https://naver.github.io/searchad-apidoc/"
)

// AccountSettings binds one social-hub account to one NAVER advertiser.
type AccountSettings struct {
	CustomerID int64 `json:"customer_id" yaml:"customer_id"`
}

// Adapter implements socialhub.Adapter for NAVER Search AD API v2.
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
		return invalidArgument("init", "product must be search-ad-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "adapter settings are not supported; the NAVER API origin is fixed")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4_096) || !validOpaque(account.SecretRef, 4_096) {
			return invalidArgument("init", "access_token_ref (API license) and secret_ref are required for every NAVER advertiser")
		}
		if account.ClientID != "" || account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "configure only access_token_ref, secret_ref, and account.settings.customer_id")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by NAVER Search AD API")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) > 0 {
			return invalidArgument("init", "OAuth approval fields are not used by NAVER Search AD API")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.CustomerID <= 0 {
			return invalidArgument("init", "account.settings.customer_id must be a positive advertiser customer ID")
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
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	apiKey, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef)
	if err != nil {
		return nil, err
	}
	secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef)
	if err != nil {
		return nil, err
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		now := resolved.Clock.Now()
		if now.UnixMilli() <= 0 {
			return invalidArgument("authenticate", "clock must return a time after the Unix epoch")
		}
		timestamp := strconv.FormatInt(now.UnixMilli(), 10)
		message := timestamp + "." + strings.ToUpper(request.Method) + "." + request.URL.EscapedPath()
		mac := hmac.New(sha256.New, []byte(token.AccessToken))
		_, _ = mac.Write([]byte(message))
		request.Header.Set("X-Timestamp", timestamp)
		request.Header.Set("X-API-KEY", apiKey)
		request.Header.Set("X-Customer", strconv.FormatInt(typed.CustomerID, 10))
		request.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		return nil
	})
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: secret, TokenType: "HMAC-SHA256"}}
	decoder := newHTTPErrorDecoder(resolved.Clock, apiKey, secret, strconv.FormatInt(typed.CustomerID, 10))
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, tokens, platformName, productName, authenticator, decoder,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(defaultBaseURL)
	return &Client{
		accountID: accountID, customerID: typed.CustomerID, api: api,
		httpClient: httpClient, baseURL: baseURL, decodeError: decoder,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: "client",
			PlatformMessage: "configured NAVER credential could not be resolved", ApprovalURL: documentationURL,
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
