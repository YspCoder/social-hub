// Package dingtalk implements DingTalk OpenAPI v1.0 for internal applications.
package dingtalk

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
	adapterName    = "dingtalk/openapi-v1.0"
	productName    = "dingtalk-openapi"
	apiVersion     = "1.0"
	defaultBaseURL = "https://api.dingtalk.com"
	docURL         = "https://open.dingtalk.com/document/orgapp-server/overview-of-server-api"
)

// Settings controls the DingTalk API origin. BaseURL overrides are intended
// for deterministic tests and approved gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings identifies an organization and an optional application bot.
type AccountSettings struct {
	CorpID    string `json:"corp_id" yaml:"corp_id"`
	RobotCode string `json:"robot_code,omitempty" yaml:"robot_code,omitempty"`
}

// Adapter implements socialhub.Adapter for DingTalk internal applications.
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

func (a *Adapter) Name() string { return adapterName }

func (a *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name: adapterName, Product: productName, APIVersion: apiVersion, DocURL: docURL,
		VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	settings := Settings{BaseURL: defaultBaseURL}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validOpaque(typed.CorpID, 256) {
			return invalidArgument("init", "account.settings.corp_id is required")
		}
		if typed.RobotCode != "" && !validOpaque(typed.RobotCode, 256) {
			return invalidArgument("init", "account.settings.robot_code is invalid")
		}
		managed := strings.TrimSpace(account.AccessTokenRef) == ""
		if managed && (!validOpaque(account.ClientID, 512) || strings.TrimSpace(account.SecretRef) == "") {
			return invalidArgument("init", "client_id and secret_ref are required when access_token_ref is not configured")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported; DingTalk Stream events are outside this adapter version")
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.settings, a.ready = config, resolved, settings, true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions, settings := a.options, a.settings
	a.mu.RUnlock()
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

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	var tokens socialhub.TokenSource
	var manager *appTokenSource
	if account.AccessTokenRef != "" {
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
		if err != nil {
			return nil, err
		}
		if !validOpaque(accessToken, 4096) {
			return nil, invalidArgument("client", "access token is invalid")
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken}}
	} else {
		secret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		if !validOpaque(secret, 4096) {
			return nil, invalidArgument("client", "client secret is invalid")
		}
		manager = &appTokenSource{
			baseURL: settings.BaseURL, corpID: typed.CorpID, clientID: account.ClientID, secret: secret,
			httpClient: &httpClient, clock: resolved.Clock, store: resolved.TokenStore,
			key: socialhub.TokenKey{Platform: "dingtalk", Product: productName, Tenant: typed.CorpID, Account: string(accountID)},
		}
		tokens = manager
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("x-acs-dingtalk-access-token", token.AccessToken)
		return nil
	})
	errorDecoder := func(status int, header http.Header, body []byte) error {
		decoded := decodeHTTPError(status, header, body)
		var platformErr *socialhub.Error
		if manager != nil && errors.As(decoded, &platformErr) && platformErr.Code == socialhub.CodeUnauthenticated {
			manager.Invalidate(context.Background())
		}
		return decoded
	}
	api, err := transport.NewWithAuthenticator(settings.BaseURL, &httpClient, tokens, "dingtalk", productName, authenticator, errorDecoder)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, robotCode: typed.RobotCode,
		api: api, tokenManager: manager, scopes: append([]string(nil), account.Approval.Scopes...),
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
