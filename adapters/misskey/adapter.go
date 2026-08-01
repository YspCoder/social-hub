// Package misskey implements the official per-instance Misskey HTTP API.
package misskey

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
	adapterName = "misskey/api"
	productName = "misskey-api"
	apiVersion  = "versionless (v2025.12+ contract)"
	docURL      = "https://misskey-hub.net/en/docs/for-developers/api/"
)

// AccountSettings identifies one Misskey instance and optionally the local
// user ID represented by the access token.
type AccountSettings struct {
	InstanceURL     string `json:"instance_url" yaml:"instance_url"`
	UserID          string `json:"user_id,omitempty" yaml:"user_id,omitempty"`
	DefaultReaction string `json:"default_reaction,omitempty" yaml:"default_reaction,omitempty"`
}

// Adapter implements socialhub.Adapter for Misskey instances.
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
	if len(config.Settings) != 0 {
		return invalidArgument("init", "Misskey instance settings belong to each account")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if account.Webhook.SecretRef != "" || account.Webhook.TokenRef != "" || account.Webhook.AESKeyRef != "" {
			return invalidArgument("init", "Misskey Streaming API is not a webhook secret contract")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validInstanceURL(typed.InstanceURL) {
			return invalidArgument("init", "account.settings.instance_url must be an absolute HTTP(S) origin")
		}
		if typed.UserID != "" && !validID(typed.UserID) {
			return invalidArgument("init", "account.settings.user_id is invalid")
		}
		if typed.DefaultReaction != "" && !validBoundedString(typed.DefaultReaction, 128) {
			return invalidArgument("init", "account.settings.default_reaction is invalid")
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
	if strings.TrimSpace(account.AccessTokenRef) == "" {
		return nil, invalidArgument("client", "access_token_ref is required")
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
	api, err := transport.New(instanceURL+"/api", resolved.HTTPClient, tokens, "misskey", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	reaction := typed.DefaultReaction
	if reaction == "" {
		reaction = "\U0001F44D"
	}
	return &Client{
		accountID: accountID, instanceURL: instanceURL, userID: typed.UserID, defaultReaction: reaction,
		api: api, permissions: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
		uploads: make(map[string]*uploadSession),
	}, nil
}

// MiAuth returns a helper for Misskey's app-registration-free authorization
// flow. It can be used before access_token_ref has been configured.
func (a *Adapter) MiAuth(accountID socialhub.AccountID) (*MiAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("miauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	options := a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformError("miauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("miauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	instanceURL := normalizeInstanceURL(typed.InstanceURL)
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "miauth-no-auth"}}
	api, err := transport.NewWithAuthenticator(
		instanceURL+"/api", options.HTTPClient, tokens, "misskey", productName,
		transport.AuthenticatorFunc(func(*http.Request, socialhub.Token) error { return nil }), decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("miauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &MiAuthClient{accountID: accountID, instanceURL: instanceURL, api: api}, nil
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

var _ socialhub.Adapter = (*Adapter)(nil)
