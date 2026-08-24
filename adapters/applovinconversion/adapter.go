// Package applovinconversion implements AppLovin Growth Conversion API v1.
package applovinconversion

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "applovin/growth-conversion-api-v1"
	platformName     = "applovin"
	productName      = "growth-conversion-api"
	apiVersion       = "v1"
	defaultBaseURL   = "https://b.applovin.com/v1"
	documentationURL = "https://support.applovin.com/en/growth/promoting-your-websites/api/conversion-api"
)

// AccountSettings binds one Conversion API key to one Event Key and policy.
type AccountSettings struct {
	EventKey string        `json:"event_key" yaml:"event_key"`
	Policy   AccountPolicy `json:"policy" yaml:"policy"`
}

// Adapter implements socialhub.Adapter for AppLovin Growth Conversion API v1.
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
		return invalidArgument("init", "product must be growth-conversion-api")
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
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validOpaque(typed.EventKey, 4096) || !validPolicy(typed.Policy) {
			return invalidArgument("init", "account.settings requires event_key and policy STANDARD, LEAD_GEN, or RESTRICTED_LEAD_GEN")
		}
		if !validOpaque(account.SecretRef, 4096) || account.ClientID != "" || account.AppID != "" ||
			account.AccessTokenRef != "" || account.TokenStore != "" {
			return invalidArgument("init", "configure only a valid secret_ref with the Conversion API key")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not supported by Conversion API v1")
		}
		if account.Approval.AccountType != "" || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "approval account type and scopes are not defined by Conversion API v1")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	apiKey, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client")
	if err != nil {
		return nil, err
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: apiKey}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if !validOpaque(token.AccessToken, 16_384) {
			return socialhub.ErrUnauthenticated
		}
		request.Header.Set("Authorization", token.AccessToken)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, httpClient, tokens, platformName, productName, authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, eventKey: typed.EventKey, policy: typed.Policy, api: api}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the AppLovin Conversion API key", err, reference)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved AppLovin Conversion API key is invalid", nil, reference, value)
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
