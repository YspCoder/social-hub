// Package applemusic implements Apple Music API v1.
package applemusic

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
	adapterName              = "applemusic/api"
	productName              = "apple-music-api"
	apiVersion               = "v1"
	defaultBaseURL           = "https://api.music.apple.com/v1"
	defaultDeveloperTokenTTL = time.Hour
	maxDeveloperTokenTTL     = 15_777_000 * time.Second
	documentationURL         = "https://developer.apple.com/documentation/applemusicapi"
)

// Settings controls the Apple Music API origin and locally generated token lifetime.
type Settings struct {
	BaseURL           string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	DeveloperTokenTTL string `json:"developer_token_ttl,omitempty" yaml:"developer_token_ttl,omitempty"`
}

// AccountSettings contains Apple-specific account credentials and defaults.
type AccountSettings struct {
	Storefront        string `json:"storefront,omitempty" yaml:"storefront,omitempty"`
	TeamID            string `json:"team_id,omitempty" yaml:"team_id,omitempty"`
	KeyID             string `json:"key_id,omitempty" yaml:"key_id,omitempty"`
	MusicUserTokenRef string `json:"music_user_token_ref,omitempty" yaml:"music_user_token_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Apple Music API v1.
type Adapter struct {
	mu       sync.RWMutex
	config   socialhub.AdapterConfig
	options  socialhub.Options
	settings Settings
	tokenTTL time.Duration
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
	settings := Settings{BaseURL: defaultBaseURL, DeveloperTokenTTL: defaultDeveloperTokenTTL.String()}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	if !validEndpoint(settings.BaseURL) {
		return invalidArgument("init", "base_url must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	tokenTTL, err := time.ParseDuration(settings.DeveloperTokenTTL)
	if err != nil || tokenTTL <= 0 || tokenTTL > maxDeveloperTokenTTL {
		return invalidArgument("init", "developer_token_ttl must be positive and no greater than 15777000 seconds")
	}
	for _, account := range config.Accounts {
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.Storefront != "" && !validStorefront(typed.Storefront) {
			return invalidArgument("init", "account.settings.storefront must be an ISO 3166 alpha-2 code")
		}
		if account.AccessTokenRef == "" {
			if account.SecretRef == "" || !validAppleIdentifier(typed.TeamID) || !validAppleIdentifier(typed.KeyID) {
				return invalidArgument("init", "team_id, key_id, and secret_ref are required when access_token_ref is absent")
			}
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return platformError("init", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	a.config, a.options, a.settings, a.tokenTTL, a.ready = config, resolved, settings, tokenTTL, true
	return nil
}

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions, settings, tokenTTL := a.options, a.settings, a.tokenTTL
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}

	var tokens socialhub.TokenSource
	if account.AccessTokenRef != "" {
		developerToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
		if err != nil || !validCredential(developerToken) {
			return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
		tokens = socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: developerToken, TokenType: "Bearer"}}
	} else {
		privateKeyPEM, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
		if err != nil {
			return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
		tokens, err = newDeveloperTokenSource(typed.TeamID, typed.KeyID, privateKeyPEM, tokenTTL, resolved.Clock)
		if err != nil {
			return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
	}
	musicUserToken, err := resolveOptionalSecret(ctx, resolved.Secrets, typed.MusicUserTokenRef)
	if err != nil || (musicUserToken != "" && !validCredential(musicUserToken)) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		request.Header.Set("Authorization", "Bearer "+token.AccessToken)
		if musicUserToken != "" {
			request.Header.Set("Music-User-Token", musicUserToken)
		}
		return nil
	})
	api, err := transport.NewWithAuthenticator(settings.BaseURL, resolved.HTTPClient, tokens, "applemusic", productName, authenticator, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	baseURL, _ := url.Parse(settings.BaseURL)
	return &Client{
		accountID: accountID, storefront: strings.ToLower(typed.Storefront), musicUserToken: musicUserToken,
		api: api, apiBaseURL: baseURL,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func resolveOptionalSecret(ctx context.Context, resolver socialhub.SecretResolver, reference string) (string, error) {
	if reference == "" {
		return "", nil
	}
	return resolver.Resolve(ctx, reference)
}

var _ socialhub.Adapter = (*Adapter)(nil)
