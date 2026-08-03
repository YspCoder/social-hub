// Package micropub implements the W3C Micropub Recommendation.
package micropub

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "indieweb/micropub"
	productName      = "micropub"
	apiVersion       = "W3C-REC-20170523"
	platformName     = "indieweb"
	documentationURL = "https://www.w3.org/TR/micropub/"
)

// AccountSettings configures one Micropub endpoint and its optional editing
// capabilities. Micropub has no standard capability query for update/delete,
// so callers must declare those server-specific features explicitly.
type AccountSettings struct {
	Endpoint         string `json:"endpoint" yaml:"endpoint"`
	SiteURL          string `json:"site_url" yaml:"site_url"`
	SupportsUpdate   bool   `json:"supports_update,omitempty" yaml:"supports_update,omitempty"`
	SupportsDelete   bool   `json:"supports_delete,omitempty" yaml:"supports_delete,omitempty"`
	SupportsUndelete bool   `json:"supports_undelete,omitempty" yaml:"supports_undelete,omitempty"`
}

// Adapter implements socialhub.Adapter for independently hosted Micropub
// endpoints.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
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
		if !validEndpoint(settings.Endpoint, true) {
			return invalidArgument("init", "account.settings.endpoint must be an absolute HTTP(S) URL without credentials or fragment")
		}
		if !validEndpoint(settings.SiteURL, true) {
			return invalidArgument("init", "account.settings.site_url must be an absolute HTTP(S) URL without credentials or fragment")
		}
		if settings.SupportsUndelete && !settings.SupportsDelete {
			return invalidArgument("init", "supports_undelete requires supports_delete")
		}
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "account.access_token_ref must reference a Micropub bearer token")
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
	endpoint, _ := url.Parse(settings.Endpoint)
	token, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if !validHeaderValue(token, maxTokenBytes) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	return &Client{
		accountID: accountID, endpoint: endpoint, siteURL: settings.SiteURL, token: token,
		httpClient: &httpClient, scopes: append([]string(nil), account.Approval.Scopes...),
		supportsUpdate: settings.SupportsUpdate, supportsDelete: settings.SupportsDelete,
		supportsUndelete: settings.SupportsUndelete, clock: resolved.Clock,
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

func validEndpoint(value string, allowQuery bool) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == "" && (allowQuery || parsed.RawQuery == "")
}

func validHeaderValue(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
