// Package bluesky implements the official Bluesky application Lexicons over AT Protocol XRPC.
package bluesky

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
	adapterName = "bluesky/atproto"
	apiVersion  = "lexicon-v1"
	productName = "atproto-xrpc"
	docURL      = "https://docs.bsky.app/docs/"
)

// AccountSettings identifies one account's PDS and repository. Identifier is
// only needed when creating a legacy session and defaults to Repo.
type AccountSettings struct {
	ServiceURL string `json:"service_url" yaml:"service_url"`
	Repo       string `json:"repo,omitempty" yaml:"repo,omitempty"`
	Identifier string `json:"identifier,omitempty" yaml:"identifier,omitempty"`
}

// Adapter implements socialhub.Adapter for Bluesky's AT Protocol APIs.
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
		Name: adapterName, Product: productName, APIVersion: apiVersion,
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
		return invalidArgument("init", "Bluesky PDS settings belong to each account")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		var settings AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil || !validServiceURL(settings.ServiceURL) {
			return invalidArgument("init", "account.settings.service_url must be an absolute HTTP(S) origin")
		}
		if settings.Repo != "" && !validDID(settings.Repo) {
			return invalidArgument("init", "account.settings.repo must be an account DID")
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
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validDID(settings.Repo) {
		return nil, invalidArgument("client", "account.settings.repo is required for repository writes")
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	httpTransport, err := transport.New(normalizeServiceURL(settings.ServiceURL), resolved.HTTPClient, tokens, "bluesky", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, serviceURL: normalizeServiceURL(settings.ServiceURL), repo: settings.Repo,
		transport: httpTransport, clock: resolved.Clock,
		uploads: make(map[string]*uploadSession), blobs: make(map[string]blobRef),
	}, nil
}

// Session returns the official legacy-session helper used by headless clients
// and bots. Full atproto OAuth requires PAR, DPoP, and identity discovery and is
// intentionally delegated to a dedicated OAuth implementation.
func (a *Adapter) Session(ctx context.Context, accountID socialhub.AccountID) (*SessionClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("session", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	options := a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformError("session", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return nil, platformError("session", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	password := ""
	if account.SecretRef != "" {
		var err error
		password, err = options.Secrets.Resolve(ctx, account.SecretRef)
		if err != nil || strings.TrimSpace(password) == "" {
			return nil, platformError("session", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
		}
	}
	identifier := settings.Identifier
	if identifier == "" {
		identifier = settings.Repo
	}
	return &SessionClient{
		ServiceURL: normalizeServiceURL(settings.ServiceURL), Identifier: identifier,
		Password: password, HTTPClient: options.HTTPClient,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func validServiceURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}

func normalizeServiceURL(value string) string { return strings.TrimRight(value, "/") }

func validDID(value string) bool {
	return strings.HasPrefix(value, "did:") && strings.Count(value, ":") >= 2 &&
		!strings.ContainsAny(value, " /?#\t\r\n")
}
