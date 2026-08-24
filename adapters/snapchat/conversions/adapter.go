// Package conversions implements Snap Conversions API v3 for one Pixel or
// Snap App asset per configured account. It is separate from Ads API campaign
// management because event ingestion has its own credentials and privacy rules.
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
	adapterName      = "snapchat/conversions-api-v3"
	productName      = "conversions-api"
	apiVersion       = "v3"
	platformName     = "snapchat"
	defaultBaseURL   = "https://tr.snapchat.com/v3"
	documentationURL = "https://developers.snap.com/marketing-api/Conversions-API/Introduction"
)

type AssetType string

const (
	AssetTypePixel   AssetType = "PIXEL"
	AssetTypeSnapApp AssetType = "SNAP_APP"
)

// AccountSettings binds one account to a Pixel or Snap App. Pixel assets may
// receive Web and Offline events; Snap App assets receive Mobile App events.
type AccountSettings struct {
	AssetID   string    `json:"asset_id" yaml:"asset_id"`
	AssetType AssetType `json:"asset_type" yaml:"asset_type"`
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
		Name: adapterName, Product: productName, APIVersion: apiVersion,
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
		return invalidArgument("init", "adapter settings are not supported; the Snap Conversions API origin is fixed")
	}
	for _, account := range config.Accounts {
		if !validOpaque(account.AccessTokenRef, 16_384) {
			return invalidArgument("init", "access_token_ref is required for every Snap CAPI asset")
		}
		if account.ClientID != "" || account.AppID != "" || account.SecretRef != "" || account.TokenStore != "" ||
			account.Webhook.SecretRef != "" || account.Webhook.TokenRef != "" || account.Webhook.AESKeyRef != "" ||
			account.Approval.AccountType != "" || len(account.Approval.Scopes) != 0 {
			return invalidArgument("init", "OAuth client, secret, token store, webhook, and approval settings are not used by this adapter")
		}
		var typed AccountSettings
		if err := socialhub.DecodeSettings(account.Settings, &typed); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if !validAssetID(typed.AssetID) || !validAssetType(typed.AssetType) {
			return invalidArgument("init", "account.settings requires a valid asset_id and asset_type PIXEL or SNAP_APP")
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
	token, err := resolveSecret(ctx, resolved.Secrets, account.AccessTokenRef, "client")
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
	tokens := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: token}}
	api, err := transport.NewWithAuthenticator(
		defaultBaseURL, &httpClient, tokens, platformName, productName,
		queryAuthenticator{}, newHTTPErrorDecoder(resolved.Clock, token, typed.AssetID),
	)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, assetID: typed.AssetID, assetType: typed.AssetType,
		api: api, clock: resolved.Clock,
	}, nil
}

type queryAuthenticator struct{}

func (queryAuthenticator) Authenticate(request *http.Request, token socialhub.Token) error {
	if !validOpaque(token.AccessToken, 16_384) {
		return socialhub.ErrUnauthenticated
	}
	query := request.URL.Query()
	query.Set("access_token", token.AccessToken)
	request.URL.RawQuery = query.Encode()
	return nil
}

func resolveSecret(ctx context.Context, resolver socialhub.SecretResolver, reference, operation string) (string, error) {
	value, err := resolver.Resolve(ctx, reference)
	if err != nil || !validOpaque(value, 16_384) {
		return "", authenticationError(operation, "Snap CAPI credential resolution failed", err, reference, value)
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
	return value != "" && value == strings.TrimSpace(value) && len(value) <= maximum && utf8.ValidString(value) &&
		!strings.ContainsFunc(value, unicode.IsControl)
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validAssetID(value string) bool {
	if !validOpaque(value, 256) {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func validAssetType(value AssetType) bool {
	return value == AssetTypePixel || value == AssetTypeSnapApp
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

var _ socialhub.Adapter = (*Adapter)(nil)
