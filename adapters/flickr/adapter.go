// Package flickr implements the official Flickr Services API.
package flickr

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dghubble/oauth1"

	"social-hub/pkg/socialhub"
)

const (
	adapterName            = "flickr/services-api"
	productName            = "services-api"
	apiVersion             = "rest"
	defaultBaseURL         = "https://api.flickr.com/services/rest"
	defaultUploadURL       = "https://up.flickr.com/services/upload/"
	defaultRequestTokenURL = "https://www.flickr.com/services/oauth/request_token"
	defaultAuthorizeURL    = "https://www.flickr.com/services/oauth/authorize"
	defaultAccessTokenURL  = "https://www.flickr.com/services/oauth/access_token"
	documentationURL       = "https://www.flickr.com/services/api/"
)

// Settings controls Flickr REST, upload, and OAuth endpoints.
type Settings struct {
	BaseURL         string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	UploadURL       string `json:"upload_url,omitempty" yaml:"upload_url,omitempty"`
	RequestTokenURL string `json:"request_token_url,omitempty" yaml:"request_token_url,omitempty"`
	AuthorizeURL    string `json:"authorize_url,omitempty" yaml:"authorize_url,omitempty"`
	AccessTokenURL  string `json:"access_token_url,omitempty" yaml:"access_token_url,omitempty"`
}

// AccountSettings identifies the Flickr member and OAuth token secret.
type AccountSettings struct {
	UserID         string `json:"user_id" yaml:"user_id"`
	TokenSecretRef string `json:"token_secret_ref,omitempty" yaml:"token_secret_ref,omitempty"`
}

// Adapter implements socialhub.Adapter for Flickr Services API.
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
	settings := Settings{
		BaseURL: defaultBaseURL, UploadURL: defaultUploadURL,
		RequestTokenURL: defaultRequestTokenURL, AuthorizeURL: defaultAuthorizeURL,
		AccessTokenURL: defaultAccessTokenURL,
	}
	if len(config.Settings) > 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.UploadURL, settings.RequestTokenURL, settings.AuthorizeURL, settings.AccessTokenURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all Flickr endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.ClientID) == "" || len(account.ClientID) > 512 {
			return invalidArgument("init", "client_id is required as the Flickr API key for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil || !validResourceID(typed.UserID) {
			return invalidArgument("init", "account.settings.user_id is required and invalid")
		}
		hasAccessToken := strings.TrimSpace(account.AccessTokenRef) != ""
		if hasAccessToken && (strings.TrimSpace(account.SecretRef) == "" || strings.TrimSpace(typed.TokenSecretRef) == "" || !validPermissionScopes(account.Approval.Scopes)) {
			return invalidArgument("init", "authenticated accounts require secret_ref, token_secret_ref, and one read/write/delete approval scope")
		}
		if !hasAccessToken && typed.TokenSecretRef != "" {
			return invalidArgument("init", "token_secret_ref requires access_token_ref")
		}
		if len(account.Approval.Scopes) > 0 && !validPermissionScopes(account.Approval.Scopes) {
			return invalidArgument("init", "Flickr approval scopes must contain exactly one of read, write, or delete")
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

func (a *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("client", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	baseOptions, settings := a.options, a.settings
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
	client := &Client{
		accountID: accountID, apiKey: account.ClientID, userID: typed.UserID,
		permission: configuredPermission(account.Approval.Scopes), baseURL: settings.BaseURL,
		uploadURL: settings.UploadURL, public: noRedirectClient(resolved.HTTPClient),
		clock: resolved.Clock,
	}
	if account.AccessTokenRef != "" {
		consumerSecret, err := resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client_consumer_secret")
		if err != nil {
			return nil, err
		}
		accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client_access_token")
		if err != nil {
			return nil, err
		}
		tokenSecret, err := resolveSecret(ctx, resolved.Secrets, typed.TokenSecretRef, "client_token_secret")
		if err != nil {
			return nil, err
		}
		client.consumerSecret, client.accessToken, client.tokenSecret = consumerSecret, accessToken, tokenSecret
		client.signed = signedHTTPClient(resolved.HTTPClient, &oauth1.Config{ConsumerKey: account.ClientID, ConsumerSecret: consumerSecret}, oauth1.NewToken(accessToken, tokenSecret))
	}
	client.upload = &PhotoUploadService{client: client}
	return client, nil
}

// OAuth returns an OAuth 1.0a helper for one Flickr application.
func (a *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	settings, options := a.settings, a.options
	a.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "secret_ref is required for OAuth")
	}
	secret, err := resolveSecret(ctx, options.Secrets, account.SecretRef, "oauth")
	if err != nil {
		return nil, err
	}
	return &OAuthClient{
		ConsumerKey: account.ClientID, ConsumerSecret: secret,
		RequestTokenURL: settings.RequestTokenURL, AuthorizeURL: settings.AuthorizeURL,
		AccessTokenURL: settings.AccessTokenURL, HTTPClient: options.HTTPClient,
	}, nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready, a.closed = false, true
	return nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || strings.TrimSpace(value) == "" {
		return "", platformError(operation, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	return value, nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validResourceID(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= 512 && !strings.ContainsAny(value, "/?#")
}

var _ socialhub.Adapter = (*Adapter)(nil)
