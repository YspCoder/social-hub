// Package kick implements the official Kick Developer Public API.
package kick

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName          = "kick/public-api-v2"
	productName          = "developer-public-api"
	apiVersion           = "v2"
	defaultBaseURL       = "https://api.kick.com"
	defaultAuthURL       = "https://id.kick.com/oauth/authorize"
	defaultTokenURL      = "https://id.kick.com/oauth/token"
	defaultRevokeURL     = "https://id.kick.com/oauth/revoke"
	defaultIntrospectURL = "https://id.kick.com/oauth/token/introspect"
	documentationURL     = "https://docs.kick.com"
)

const defaultWebhookPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAq/+l1WnlRrGSolDMA+A8
6rAhMbQGmQ2SapVcGM3zq8ANXjnhDWocMqfWcTd95btDydITa10kDvHzw9WQOqp2
MZI7ZyrfzJuz5nhTPCiJwTwnEtWft7nV14BYRDHvlfqPUaZ+1KR4OCaO/wWIk/rQ
L/TjY0M70gse8rlBkbo2a8rKhu69RQTRsoaf4DVhDPEeSeI5jVrRDGAMGL3cGuyY
6CLKGdjVEM78g3JfYOvDU/RvfqD7L89TZ3iN94jrmWdGz34JNlEI5hqK8dd7C5EF
BEbZ5jgB8s8ReQV8H+MkuffjdAj3ajDDX3DOJMIut1lBrUVD1AaSrGCKHooWoL2e
twIDAQAB
-----END PUBLIC KEY-----`

// Settings controls Kick API and OAuth endpoints.
type Settings struct {
	BaseURL       string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	AuthURL       string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	TokenURL      string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	RevokeURL     string `json:"revoke_url,omitempty" yaml:"revoke_url,omitempty"`
	IntrospectURL string `json:"introspect_url,omitempty" yaml:"introspect_url,omitempty"`
}

// AccountSettings describes the token and optional broadcaster represented by an account.
type AccountSettings struct {
	BroadcasterUserID string `json:"broadcaster_user_id,omitempty" yaml:"broadcaster_user_id,omitempty"`
	ChannelSlug       string `json:"channel_slug,omitempty" yaml:"channel_slug,omitempty"`
	TokenType         string `json:"token_type" yaml:"token_type"`
	WebhookPublicKey  string `json:"webhook_public_key,omitempty" yaml:"webhook_public_key,omitempty"`
}

// Adapter implements socialhub.Adapter for Kick's public developer API.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
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
		BaseURL: defaultBaseURL, AuthURL: defaultAuthURL, TokenURL: defaultTokenURL,
		RevokeURL: defaultRevokeURL, IntrospectURL: defaultIntrospectURL,
	}
	if len(config.Settings) != 0 {
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, endpoint := range []string{settings.BaseURL, settings.AuthURL, settings.TokenURL, settings.RevokeURL, settings.IntrospectURL} {
		if !validEndpoint(endpoint) {
			return invalidArgument("init", "all Kick endpoints must be absolute HTTP(S) URLs without credentials, query, or fragment")
		}
	}
	for _, account := range config.Accounts {
		if strings.TrimSpace(account.AccessTokenRef) == "" {
			return invalidArgument("init", "access_token_ref is required for every account")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if typed.TokenType != "app" && typed.TokenType != "user" {
			return invalidArgument("init", "account.settings.token_type must be app or user")
		}
		if typed.BroadcasterUserID != "" && !validPositiveID(typed.BroadcasterUserID) {
			return invalidArgument("init", "account.settings.broadcaster_user_id must be a positive decimal integer")
		}
		if typed.ChannelSlug != "" && !validSlug(typed.ChannelSlug) {
			return invalidArgument("init", "account.settings.channel_slug must be 1-25 safe characters")
		}
		publicKey := typed.WebhookPublicKey
		if strings.TrimSpace(publicKey) == "" {
			publicKey = defaultWebhookPublicKey
		}
		if _, err := parseWebhookPublicKey(publicKey); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
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

func (adapter *Adapter) Client(ctx context.Context, accountID socialhub.AccountID, options ...socialhub.Option) (socialhub.Client, error) {
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
	accessToken, err := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
	if err != nil || !validBearerToken(accessToken) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	publicKeyPEM := typed.WebhookPublicKey
	if strings.TrimSpace(publicKeyPEM) == "" {
		publicKeyPEM = defaultWebhookPublicKey
	}
	publicKey, err := parseWebhookPublicKey(publicKeyPEM)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: strings.TrimSpace(accessToken), TokenType: "Bearer"}}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectCrossOriginRedirect
	api, err := transport.New(settings.BaseURL, &httpClient, tokens, "kick", productName, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, broadcasterUserID: typed.BroadcasterUserID, channelSlug: typed.ChannelSlug,
		tokenType: typed.TokenType, scopes: append([]string(nil), account.Approval.Scopes...), api: api,
		webhookPublicKey: publicKey,
	}, nil
}

// OAuth returns an OAuth 2.1 helper for the configured Kick application.
func (adapter *Adapter) OAuth(ctx context.Context, accountID socialhub.AccountID) (*OAuthClient, error) {
	adapter.mu.RLock()
	if !adapter.ready || adapter.closed {
		adapter.mu.RUnlock()
		return nil, platformError("oauth", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := adapter.config.Account(accountID)
	settings, options := adapter.settings, adapter.options
	adapter.mu.RUnlock()
	if !found {
		return nil, platformError("oauth", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if strings.TrimSpace(account.ClientID) == "" || strings.TrimSpace(account.SecretRef) == "" {
		return nil, invalidArgument("oauth", "client_id and secret_ref are required")
	}
	secret, err := options.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || strings.TrimSpace(secret) == "" {
		return nil, platformError("oauth", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	httpClient := *options.HTTPClient
	httpClient.CheckRedirect = rejectCrossOriginRedirect
	return &OAuthClient{
		ClientID: account.ClientID, ClientSecret: secret, AuthURL: settings.AuthURL, TokenURL: settings.TokenURL,
		RevokeURL: settings.RevokeURL, IntrospectURL: settings.IntrospectURL,
		HTTPClient: &httpClient, Clock: options.Clock,
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validBearerToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 16384 {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func validPositiveID(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func validSlug(value string) bool {
	if strings.TrimSpace(value) != value || len(value) == 0 || len(value) > 25 {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f || strings.ContainsRune("/?#\\", character) {
			return false
		}
	}
	return true
}

func rejectCrossOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	origin := via[0].URL
	if !strings.EqualFold(request.URL.Scheme, origin.Scheme) || !strings.EqualFold(request.URL.Host, origin.Host) {
		return http.ErrUseLastResponse
	}
	if len(via) >= 10 {
		return errors.New("kick: too many redirects")
	}
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
