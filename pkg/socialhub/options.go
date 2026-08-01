package socialhub

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Clock makes token expiry and rate-limit behavior deterministic in tests.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options contains dependencies shared by adapters.
type Options struct {
	HTTPClient *http.Client
	Logger     *slog.Logger
	TokenStore TokenStore
	Secrets    SecretResolver
	Clock      Clock
}

// Option configures an adapter or client.
type Option func(*Options) error

// ResolveOptions returns validated options with conservative defaults.
func ResolveOptions(options ...Option) (Options, error) {
	resolved := Options{
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		Logger:     slog.New(slog.DiscardHandler),
		Secrets:    EnvironmentSecretResolver{},
		Clock:      systemClock{},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&resolved); err != nil {
			return Options{}, err
		}
	}
	return resolved, nil
}

// WithSecretResolver supplies the resolver for credential references.
func WithSecretResolver(resolver SecretResolver) Option {
	return func(options *Options) error {
		if resolver == nil {
			return errors.New("socialhub: secret resolver must not be nil")
		}
		options.Secrets = resolver
		return nil
	}
}

// WithHTTPClient supplies the HTTP client used for platform requests.
func WithHTTPClient(client *http.Client) Option {
	return func(options *Options) error {
		if client == nil {
			return errors.New("socialhub: HTTP client must not be nil")
		}
		options.HTTPClient = client
		return nil
	}
}

// WithLogger supplies a structured logger.
func WithLogger(logger *slog.Logger) Option {
	return func(options *Options) error {
		if logger == nil {
			return errors.New("socialhub: logger must not be nil")
		}
		options.Logger = logger
		return nil
	}
}

// WithTokenStore supplies persistent token storage.
func WithTokenStore(store TokenStore) Option {
	return func(options *Options) error {
		if store == nil {
			return errors.New("socialhub: token store must not be nil")
		}
		options.TokenStore = store
		return nil
	}
}

// WithClock supplies a clock used for expiry calculations.
func WithClock(clock Clock) Option {
	return func(options *Options) error {
		if clock == nil {
			return errors.New("socialhub: clock must not be nil")
		}
		options.Clock = clock
		return nil
	}
}
