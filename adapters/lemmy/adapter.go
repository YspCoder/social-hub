// Package lemmy implements the official Lemmy HTTP API v3 used by Lemmy 0.19.x.
package lemmy

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "lemmy/api-v3"
	productName      = "api"
	apiVersion       = "v3/0.19.x"
	documentationURL = "https://join-lemmy.org/docs/contributors/04-api.html"
)

// AccountSettings identifies one user on one Lemmy instance.
type AccountSettings struct {
	BaseURL  string `json:"base_url" yaml:"base_url"`
	Username string `json:"username" yaml:"username"`
}

// Adapter implements socialhub.Adapter for Lemmy API v3.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if len(config.Settings) != 0 {
		var settings struct{}
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		var settings AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validBaseURL(settings.BaseURL) {
			return invalidArgument("init", "account.settings.base_url must be an HTTP(S) Lemmy origin without credentials, path, query, or fragment")
		}
		if !validUsername(settings.Username) {
			return invalidArgument("init", "account.settings.username must identify the configured Lemmy user")
		}
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "account.access_token_ref must reference a Lemmy 0.19 JWT")
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
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	jwt, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	jwt = strings.TrimSpace(jwt)
	if !validHeaderValue(jwt, 16384) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectCrossOriginRedirect
	api, err := transport.New(
		baseURL, &httpClient,
		socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: jwt, TokenType: "Bearer"}},
		"lemmy", productName, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, baseURL: baseURL, username: settings.Username,
		api: api, clock: resolved.Clock, uploads: make(map[string]*uploadState), media: make(map[string]socialhub.Media),
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
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

func validBaseURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && (parsed.Path == "" || parsed.Path == "/") && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validHeaderValue(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func rejectCrossOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	origin := via[0].URL
	if !strings.EqualFold(request.URL.Scheme, origin.Scheme) || !strings.EqualFold(request.URL.Host, origin.Host) {
		return http.ErrUseLastResponse
	}
	if len(via) >= 10 {
		return errors.New("lemmy: too many redirects")
	}
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
