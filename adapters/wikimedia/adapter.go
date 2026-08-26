// Package wikimedia implements anonymous, read-only MediaWiki REST API v1
// workflows for Wikipedia language editions and Wikimedia Commons.
package wikimedia

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "wikimedia/mediawiki-rest-v1"
	platformName     = "wikimedia"
	productName      = "mediawiki-rest"
	apiVersion       = "v1"
	documentationURL = "https://www.mediawiki.org/wiki/API:REST_API/Reference"
)

// Settings contains the application identity required by Wikimedia's
// User-Agent policy.
type Settings struct {
	UserAgent string `json:"user_agent" yaml:"user_agent"`
}

type Project string

const (
	ProjectWikipedia Project = "wikipedia"
	ProjectCommons   Project = "commons"
)

// AccountSettings selects one official Wikimedia site. Language is required
// for Wikipedia and must be omitted for Commons.
type AccountSettings struct {
	Project  Project `json:"project" yaml:"project"`
	Language string  `json:"language,omitempty" yaml:"language,omitempty"`
}

// Adapter implements socialhub.Adapter for MediaWiki REST API v1.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be mediawiki-rest")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	var settings Settings
	if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "settings.user_agent must identify the client and contain contact information")
	}
	for _, account := range config.Accounts {
		if !publicAccountOnly(account) {
			return invalidArgument("init", "anonymous Wikimedia accounts accept only id and project settings")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validAccountSettings(typed) {
			return invalidArgument("init", "account settings must select wikipedia with a language or commons without a language")
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

func (adapter *Adapter) Client(_ context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := cloneHTTPClient(resolved.HTTPClient)
	return &Client{
		accountID: accountID, project: typed.Project, language: typed.Language,
		baseURL: siteBaseURL(typed), userAgent: settings.UserAgent,
		httpClient: httpClient, clock: resolved.Clock,
	}, nil
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
