// Package peertube implements the official per-instance PeerTube REST API v1.
package peertube

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
	adapterName = "peertube/rest-v1"
	productName = "peertube-rest-api"
	apiVersion  = "v1"
	docURL      = "https://docs.joinpeertube.org/api-rest-reference"
)

// Settings is intentionally empty. PeerTube is federated, so the instance URL
// belongs to each configured account rather than to the adapter as a whole.
type Settings struct{}

// AccountSettings selects one PeerTube instance and an optional default actor.
type AccountSettings struct {
	InstanceURL string `json:"instance_url" yaml:"instance_url"`
	AccountName string `json:"account_name,omitempty" yaml:"account_name,omitempty"`
}

// Adapter implements socialhub.Adapter for PeerTube REST API v1.
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
	var settings Settings
	if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		if !validRoles(account.Approval.Scopes) {
			return invalidArgument("init", "approval scopes must contain unique PeerTube roles: user, moderator, or admin")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validInstanceURL(typed.InstanceURL) {
			return invalidArgument("init", "account.settings.instance_url must be an absolute HTTP(S) origin")
		}
		if typed.AccountName != "" && !validActorHandle(typed.AccountName) {
			return invalidArgument("init", "account.settings.account_name is invalid")
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
	if baseOptions.TokenStore != nil {
		combined = append(combined, socialhub.WithTokenStore(baseOptions.TokenStore))
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(account.AccessTokenRef) == "" {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, socialhub.ErrUnauthenticated)
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
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{
		AccessToken: accessToken, TokenType: "Bearer", Scopes: append([]string(nil), account.Approval.Scopes...),
	}}
	api, err := transport.New(instanceURL+"/api/v1", resolved.HTTPClient, tokens, "peertube", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, accountName: typed.AccountName, instanceURL: instanceURL,
		transport: api, roles: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
	}, nil
}

// OAuth returns an OAuth password/refresh grant helper for one account's instance.
func (a *Adapter) OAuth(accountID socialhub.AccountID) (*OAuthClient, error) {
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("oauth", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &OAuthClient{InstanceURL: normalizeInstanceURL(typed.InstanceURL), HTTPClient: options.HTTPClient, Clock: options.Clock}, nil
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

func validRoles(roles []string) bool {
	seen := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		switch role {
		case "user", "moderator", "admin":
		default:
			return false
		}
		if _, exists := seen[role]; exists {
			return false
		}
		seen[role] = struct{}{}
	}
	return true
}
