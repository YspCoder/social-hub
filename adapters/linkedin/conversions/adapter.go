// Package conversions implements LinkedIn Conversions API event ingestion for
// Marketing API version 202608. Conversion telemetry remains separate from
// organic LinkedIn and paid-media management adapters.
package conversions

import (
	"context"
	"net/http"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "linkedin/conversions-api-202608"
	platformName     = "linkedin"
	productName      = "conversions-api"
	marketingVersion = "202608"
	restliVersion    = "2.0.0"
	defaultBaseURL   = "https://api.linkedin.com/rest"
	documentationURL = "https://learn.microsoft.com/en-us/linkedin/marketing/integrations/ads-reporting/conversions-api?view=li-lms-2026-08"
	approvalURL      = "https://learn.microsoft.com/en-us/linkedin/marketing/conversions/getting-access-conversions?view=li-lms-2026-08"
	writeScope       = "rw_conversions"
	readAdsScope     = "r_ads"
)

// AccountSettings binds one social-hub account to one enabled LinkedIn
// Conversion Rule in one Sponsored Ad Account.
type AccountSettings struct {
	AdAccountID  string `json:"ad_account_id" yaml:"ad_account_id"`
	ConversionID string `json:"conversion_id" yaml:"conversion_id"`
}

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
		Name: adapterName, Product: productName, APIVersion: marketingVersion,
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
			return invalidArgument("init", "access_token_ref is required for every LinkedIn Conversion Rule")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" ||
			account.Webhook != (socialhub.WebhookConfig{}) {
			return invalidArgument("init", "OAuth client, secret, token store, app, and webhook settings are not used by this adapter")
		}
		if account.Approval.AccountType != "" || !validApprovalScopes(account.Approval.Scopes) {
			return invalidArgument("init", "approval.account_type must be empty and scopes may only contain rw_conversions and r_ads")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validNumericID(typed.AdAccountID) || !validNumericID(typed.ConversionID) {
			return invalidArgument("init", "account.settings.ad_account_id and conversion_id must be nonzero numeric IDs")
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
	var typed AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	httpClient.Jar = nil
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: accessToken, TokenType: "Bearer"}}
	authenticator := transport.AuthenticatorFunc(func(request *http.Request, token socialhub.Token) error {
		if err := (transport.BearerAuthenticator{}).Authenticate(request, token); err != nil {
			return err
		}
		request.Header.Set("Linkedin-Version", marketingVersion)
		request.Header.Set("X-Restli-Protocol-Version", restliVersion)
		return nil
	})
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient, tokens, platformName, productName,
		authenticator, newHTTPErrorDecoder(resolved.Clock),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, adAccountID: typed.AdAccountID,
		conversionURN: conversionURNPrefix + typed.ConversionID, api: api,
		scopes: append([]string(nil), account.Approval.Scopes...), clock: resolved.Clock,
	}, nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil {
		return "", authenticationError(operation, "could not resolve the LinkedIn access token", err, value)
	}
	if !validOpaque(value, 8192) {
		return "", authenticationError(operation, "resolved LinkedIn access token is invalid", nil, value)
	}
	return value, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
