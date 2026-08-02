// Package tvmaze implements the unversioned public TVmaze API.
package tvmaze

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "tvmaze/public-api"
	productName      = "tvmaze-api"
	apiVersion       = "unversioned"
	defaultBaseURL   = "https://api.tvmaze.com"
	defaultUserAgent = "social-hub/tvmaze"
	documentationURL = "https://www.tvmaze.com/api"
)

// Settings controls the public API endpoint and required client identification.
type Settings struct {
	BaseURL   string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	UserAgent string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

// Adapter implements socialhub.Adapter for TVmaze's public API.
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
		Name: adapterName, Product: productName, APIVersion: apiVersion,
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
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
	settings := Settings{BaseURL: defaultBaseURL, UserAgent: defaultUserAgent}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) || !validUserAgent(settings.UserAgent) {
		return invalidArgument("init", "base_url or user_agent is invalid")
	}
	for _, account := range config.Accounts {
		if !publicAccountOnly(account) {
			return invalidArgument("init", "TVmaze public accounts accept only an id")
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

func (a *Adapter) Client(_ context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	_, found := a.config.Account(accountID)
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

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, _ socialhub.Token) error {
		request.Header.Set("User-Agent", settings.UserAgent)
		return nil
	})
	tokenSource := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "public"}}
	api, err := transport.NewWithAuthenticator(settings.BaseURL, &httpClient, tokenSource, "tvmaze", productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, err := url.Parse(settings.BaseURL)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api, baseURL: baseURL}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
