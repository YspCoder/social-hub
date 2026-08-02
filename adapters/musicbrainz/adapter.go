// Package musicbrainz implements MusicBrainz Web Service version 2.
package musicbrainz

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName            = "musicbrainz/ws-v2"
	productName            = "musicbrainz-web-service"
	apiVersion             = "2"
	defaultBaseURL         = "https://musicbrainz.org/ws/2"
	defaultUserAgent       = "social-hub/0.1 (https://github.com/YspCoder/social-hub)"
	defaultRequestInterval = "1100ms"
	documentationURL       = "https://musicbrainz.org/doc/MusicBrainz_API"
)

// Settings controls the API endpoint, client identification, and shared request cadence.
type Settings struct {
	BaseURL         string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	UserAgent       string `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	RequestInterval string `json:"request_interval,omitempty" yaml:"request_interval,omitempty"`
}

// Adapter implements socialhub.Adapter for MusicBrainz WS/2.
type Adapter struct {
	mu       sync.RWMutex
	config   socialhub.AdapterConfig
	options  socialhub.Options
	settings Settings
	gate     *requestGate
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
	settings := Settings{
		BaseURL: defaultBaseURL, UserAgent: defaultUserAgent, RequestInterval: defaultRequestInterval,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	interval, err := time.ParseDuration(settings.RequestInterval)
	if !validEndpoint(settings.BaseURL) || !validUserAgent(settings.UserAgent) || err != nil || interval < 0 || interval > time.Minute {
		return invalidArgument("init", "base_url, user_agent, or request_interval is invalid")
	}
	for _, account := range config.Accounts {
		if !publicAccountOnly(account) {
			return invalidArgument("init", "MusicBrainz public accounts accept only an id")
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.settings = config, resolved, settings
	a.gate, a.ready = newRequestGate(interval), true
	return nil
}

func (a *Adapter) Client(_ context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	_, found := a.config.Account(accountID)
	baseOptions, settings, gate := a.options, a.settings, a.gate
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
		query := request.URL.Query()
		query.Set("fmt", "json")
		request.URL.RawQuery = query.Encode()
		request.Header.Set("User-Agent", settings.UserAgent)
		return nil
	})
	tokenSource := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "public"}}
	api, err := transport.NewWithAuthenticator(settings.BaseURL, &httpClient, tokenSource, "musicbrainz", productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{accountID: accountID, api: api, gate: gate}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
