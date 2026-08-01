// Package mastodon implements the official Mastodon REST API.
package mastodon

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName = "mastodon/rest"
	apiVersion  = "v1+v2"
	docURL      = "https://docs.joinmastodon.org/methods/"
)

// AccountSettings identifies a Mastodon instance and optionally the local
// account ID authorized by the token.
type AccountSettings struct {
	InstanceURL string `json:"instance_url" yaml:"instance_url"`
	UserID      string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
}

// Adapter implements socialhub.Adapter for Mastodon REST API instances.
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

func (a *Adapter) Name() string { return adapterName }

func (a *Adapter) Metadata() socialhub.Metadata {
	return socialhub.Metadata{
		Name: adapterName, Product: "mastodon-rest-api", APIVersion: apiVersion,
		DocURL: docURL, VerifiedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (a *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if len(config.Settings) > 0 {
		return invalidArgument("init", "Mastodon instance settings belong to each account")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil || !validInstanceURL(typed.InstanceURL) {
			return invalidArgument("init", "account.settings.instance_url must be an absolute HTTP(S) origin")
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.ready = config, resolved, true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions := a.options
	a.mu.RUnlock()
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
	if account.AccessTokenRef == "" {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || strings.TrimSpace(accessToken) == "" {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	instanceURL := normalizeInstanceURL(typed.InstanceURL)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	httpTransport, err := transport.New(instanceURL, resolved.HTTPClient, tokens, "mastodon", "mastodon-rest-api", decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, instanceURL: instanceURL, userID: typed.UserID,
		transport: httpTransport, scopes: append([]string(nil), account.Approval.Scopes...),
		clock: resolved.Clock, uploads: make(map[string]*uploadSession),
	}, nil
}

// OAuth returns an OAuth 2.0 helper for one configured Mastodon application.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	options := a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if account.ClientID == "" || account.SecretRef == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || strings.TrimSpace(secret) == "" {
		return nil, platformError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &OAuthClient{
		InstanceURL: normalizeInstanceURL(typed.InstanceURL), ClientID: account.ClientID,
		ClientSecret: secret, HTTPClient: options.HTTPClient,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func validInstanceURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}

func normalizeInstanceURL(value string) string { return strings.TrimRight(value, "/") }
