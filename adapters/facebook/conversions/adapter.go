// Package conversions implements Meta Conversions API v26 for one Pixel or
// dataset per configured account. It is intentionally separate from organic
// Facebook publishing and Marketing API resource management.
package conversions

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "facebook/conversions-api-v26"
	productName      = "conversions-api"
	graphVersion     = "v26.0"
	platformName     = "facebook"
	defaultBaseURL   = "https://graph.facebook.com/v26.0"
	documentationURL = "https://developers.facebook.com/docs/marketing-api/conversions-api/"
	approvalURL      = "https://business.facebook.com/events_manager2/"
	managementScope  = "ads_management"
)

// AccountSettings binds a social-hub account to one Meta Pixel or dataset.
type AccountSettings struct {
	PixelID string `json:"pixel_id" yaml:"pixel_id"`
}

// Adapter implements socialhub.Adapter for Meta Conversions API v26.
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
		Name: adapterName, Product: productName, APIVersion: graphVersion,
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC),
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
		return invalidArgument("init", "product must be conversions-api")
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	if len(config.Settings) > 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 4096) {
			return invalidArgument("init", "access_token_ref is required for every Meta dataset")
		}
		if !validOptionalOpaque(account.SecretRef, 4096) {
			return invalidArgument("init", "secret_ref must be empty or a valid app-secret reference")
		}
		if account.ClientID != "" || account.AppID != "" || account.TokenStore != "" {
			return invalidArgument("init", "OAuth client and token store settings are not used by this adapter")
		}
		if account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "webhook settings are not used by this adapter")
		}
		if account.Approval.AccountType != "" || !validApprovalScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval.account_type must be empty and scopes may only contain ads_management")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validNumericID(typed.PixelID) {
			return invalidArgument("init", "account.settings.pixel_id must be a numeric Pixel or dataset ID")
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
	accessToken, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
	if err != nil {
		return nil, err
	}
	appSecret := ""
	if strings.TrimSpace(account.SecretRef) != "" {
		appSecret, err = resolveSecret(ctx, resolved.Secrets, account.SecretRef, "client_app_secret")
		if err != nil {
			return nil, err
		}
	}
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	httpClient.Jar = nil
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient, tokens, platformName, productName,
		graphAuthenticator{appSecret: appSecret}, decodeHTTPError,
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, pixelID: typed.PixelID, api: api,
		scopes: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the Meta credential", err, value)
	}
	if !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "resolved Meta credential is invalid", nil, value)
	}
	return value, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

func validOpaque(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validNumericID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validApprovalScopes(scopes []string) bool {
	if len(scopes) > 1 {
		return false
	}
	return len(scopes) == 0 || scopes[0] == managementScope
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
