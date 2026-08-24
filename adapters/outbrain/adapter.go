// Package outbrain implements marketer-scoped Outbrain Amplify API v0.1 workflows.
// Paid-media resources remain separate from social-hub's organic interfaces.
package outbrain

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
	adapterName      = "outbrain/amplify-api-v0.1"
	platformName     = "outbrain"
	productName      = "amplify-api"
	apiVersion       = "0.1"
	defaultBaseURL   = "https://api.outbrain.com/amplify/v0.1"
	documentationURL = "https://amplifyv01.docs.apiary.io/"
)

// Settings controls the Amplify API endpoint. Overrides are intended for
// tests and controlled gateways.
type Settings struct {
	BaseURL string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
}

// AccountSettings binds one social-hub account to an Outbrain marketer.
// Username is required only when the adapter obtains tokens through /login.
type AccountSettings struct {
	MarketerID string `json:"marketer_id" yaml:"marketer_id"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
}

// Adapter implements socialhub.Adapter for Outbrain Amplify API v0.1.
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
		return invalidArgument("init", "product must be amplify-api")
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
	if !validEndpoint(settings.BaseURL) || strings.HasSuffix(settings.BaseURL, "/") {
		return invalidArgument("init", "settings.base_url must be an absolute HTTP(S) URL without credentials, query, fragment, or trailing slash")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validPathID(typed.MarketerID) {
			return invalidArgument("init", "account.settings.marketer_id is invalid")
		}
		staticToken := strings.TrimSpace(account.AccessTokenRef) != ""
		login := validOpaque(typed.Username, 1024) && strings.TrimSpace(account.SecretRef) != ""
		if staticToken == login {
			return invalidArgument("init", "configure exactly one of access_token_ref or settings.username with secret_ref")
		}
		if staticToken && (typed.Username != "" || account.SecretRef != "") {
			return invalidArgument("init", "settings.username and secret_ref cannot be combined with access_token_ref")
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
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "OB-TOKEN-V1"}}
	} else {
		password, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
		if err != nil {
			return nil, err
		}
		tokens = &loginTokenSource{
			login: LoginClient{Username: typed.Username, Password: password, BaseURL: settings.BaseURL, HTTPClient: httpClient, Clock: resolved.Clock},
			store: resolved.TokenStore,
			key:   socialhub.TokenKey{Platform: platformName, Product: productName, Tenant: typed.Username, Account: string(accountID)},
		}
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validOpaque(token.AccessToken, 8192) {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("OB-TOKEN-V1", token.AccessToken)
		return nil
	})
	api, err := transport.NewWithAuthenticator(settings.BaseURL, httpClient, tokens, platformName, productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, marketerID: typed.MarketerID, api: api}, nil
}

// Login returns an Outbrain Basic-authentication /login helper.
func (adapter *Adapter) Login(ctx context.Context, accountID socialhub.AccountID) (*LoginClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("login", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	settings, options := adapter.settings, adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("login", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("login", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validOpaque(typed.Username, 1024) || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("login", "settings.username and secret_ref are required")
	}
	password, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "login")
	if err != nil {
		return nil, err
	}
	return &LoginClient{
		Username: typed.Username, Password: password, BaseURL: settings.BaseURL,
		HTTPClient: cloneHTTPClient(options.HTTPClient), Clock: options.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 8192) {
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

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

var _ socialhub.Adapter = (*Adapter)(nil)
