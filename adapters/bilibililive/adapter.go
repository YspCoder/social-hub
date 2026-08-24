// Package bilibililive implements Bilibili Live Open Platform project sessions
// and message streams.
package bilibililive

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "bilibili/live-open-platform-v2"
	productName      = "live-open-platform"
	apiVersion       = "v2-lifecycle/protocol-v1"
	defaultBaseURL   = "https://live-open.biliapi.com"
	documentationURL = "https://open-live.bilibili.com/document/eba8e2e1-847d-e908-2e5c-7a1ec7d9266f"
)

// Adapter creates clients for Bilibili Live Open Platform applications.
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
		DocURL: documentationURL, VerifiedAt: time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC),
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
	if len(config.Settings) != 0 {
		var empty struct{}
		if err := socialhub.DecodeSettings(config.Settings, &empty); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for _, account := range config.Accounts {
		if !validCredentialID(account.ClientID) {
			return invalidArgument("init", "client_id must contain the Bilibili AccessKeyId for every account")
		}
		if strings.TrimSpace(account.SecretRef) == "" {
			return invalidArgument("init", "secret_ref is required for every account")
		}
		if _, err := parseAppID(account.AppID); err != nil {
			return invalidArgument("init", "app_id must be a positive signed 64-bit decimal integer")
		}
		if len(account.Settings) != 0 {
			var empty struct{}
			if err := socialhub.DecodeSettings(account.Settings, &empty); err != nil {
				return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
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
		socialhub.WithHTTPClient(baseOptions.HTTPClient),
		socialhub.WithLogger(baseOptions.Logger),
		socialhub.WithSecretResolver(baseOptions.Secrets),
		socialhub.WithClock(baseOptions.Clock),
	}
	combined = append(combined, options...)
	resolved, err := socialhub.ResolveOptions(combined...)
	if err != nil {
		return nil, err
	}
	secret, err := resolved.Secrets.Resolve(ctx, account.SecretRef)
	if err != nil || !validSecret(secret) {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, err)
	}
	appID, err := parseAppID(account.AppID)
	if err != nil {
		return nil, invalidArgument("client", "app_id must be a positive signed 64-bit decimal integer")
	}

	httpClient := *resolved.HTTPClient
	httpClient.CheckRedirect = rejectRedirect
	httpClient.Jar = nil
	signer := &requestSigner{accessKeyID: strings.TrimSpace(account.ClientID), accessKeySecret: strings.TrimSpace(secret), clock: resolved.Clock}
	dummyToken := socialhub.StaticTokenSource{Value: socialhub.Token{AccessToken: "signed-request"}}
	api, err := transport.NewWithAuthenticator(defaultBaseURL, &httpClient, dummyToken, "bilibili", productName, signer, decodeHTTPError)
	if err != nil {
		return nil, platformError("client", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return &Client{
		accountID: accountID, appID: appID, api: api, httpClient: &httpClient,
		logger: resolved.Logger, clock: resolved.Clock, streams: make(map[*MessageStream]struct{}),
	}, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

func parseAppID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid app ID")
	}
	return parsed, nil
}

func validCredentialID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validSecret(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 4096 && !strings.ContainsAny(value, "\r\n\x00")
}

func rejectRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

var _ socialhub.Adapter = (*Adapter)(nil)
