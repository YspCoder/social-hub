// Package nostr implements the decentralized Nostr relay protocol defined by NIP-01.
package nostr

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	nostrgo "fiatjaf.com/nostr"

	"social-hub/pkg/socialhub"
)

const (
	adapterName      = "nostr/nip-01"
	productName      = "relay-protocol"
	apiVersion       = "NIP-01"
	documentationURL = "https://github.com/nostr-protocol/nips/blob/master/01.md"
)

// AccountSettings configures the relays and public identity for one account.
// AccessTokenRef, when present, must resolve to an nsec or a 64-character hex
// private key. PublicKey is required for read-only accounts.
type AccountSettings struct {
	RelayURLs   []string `json:"relay_urls" yaml:"relay_urls"`
	PublicKey   string   `json:"public_key,omitempty" yaml:"public_key,omitempty"`
	WriteQuorum int      `json:"write_quorum,omitempty" yaml:"write_quorum,omitempty"`
}

// Adapter creates Nostr clients for configured identities and relay sets.
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
		SDKVersion: "fiatjaf.com/nostr@v0.0.0-20260731140316-a8080728893f", DocURL: documentationURL,
		VerifiedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	}
}

func (adapter *Adapter) Init(_ context.Context, config socialhub.AdapterConfig, options ...socialhub.Option) error {
	if err := config.Validate(); err != nil {
		return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if config.Adapter != adapterName {
		return invalidArgument("init", "adapter name mismatch")
	}
	if len(config.Settings) != 0 {
		var settings struct{}
		if err := socialhub.DecodeSettings(config.Settings, &settings); err != nil {
			return platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	resolved, err := socialhub.ResolveOptions(options...)
	if err != nil {
		return err
	}
	for _, account := range config.Accounts {
		settings, err := decodeAccountSettings(account)
		if err != nil {
			return err
		}
		if strings.TrimSpace(account.AccessTokenRef) == "" && strings.TrimSpace(settings.PublicKey) == "" {
			return invalidArgument("init", "read-only accounts require account.settings.public_key")
		}
		if settings.PublicKey != "" {
			if _, err := parsePublicKey(settings.PublicKey); err != nil {
				return invalidArgument("init", "account.settings.public_key must be hex, npub, or nprofile")
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
	settings, err := decodeAccountSettings(account)
	if err != nil {
		return nil, err
	}

	var secret *nostrgo.SecretKey
	var publicKey nostrgo.PubKey
	if strings.TrimSpace(account.AccessTokenRef) != "" {
		credential, resolveErr := resolved.Secrets.Resolve(ctx, account.AccessTokenRef)
		if resolveErr != nil || strings.TrimSpace(credential) == "" {
			return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, resolveErr)
		}
		parsed, parseErr := parseSecretKey(credential)
		if parseErr != nil {
			return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
		}
		secret, publicKey = &parsed, parsed.Public()
	}
	if settings.PublicKey != "" {
		configured, parseErr := parsePublicKey(settings.PublicKey)
		if parseErr != nil {
			return nil, invalidArgument("client", "account.settings.public_key is invalid")
		}
		if secret != nil && configured != publicKey {
			return nil, invalidArgument("client", "configured public key does not match the private key")
		}
		publicKey = configured
	}
	if publicKey == nostrgo.ZeroPK {
		return nil, platformError("client", socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
	}

	network := newRelayNetwork(settings.RelayURLs, resolved.HTTPClient)
	return &Client{
		accountID: accountID, publicKey: publicKey, secretKey: secret,
		relays: append([]string(nil), settings.RelayURLs...), writeQuorum: settings.WriteQuorum,
		network: network, clock: resolved.Clock,
	}, nil
}

func decodeAccountSettings(account socialhub.AccountConfig) (AccountSettings, error) {
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return AccountSettings{}, platformError("init", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	var err error
	settings.RelayURLs, err = normalizeRelayURLs(settings.RelayURLs)
	if err != nil {
		return AccountSettings{}, invalidArgument("init", err.Error())
	}
	if len(settings.RelayURLs) == 0 {
		return AccountSettings{}, invalidArgument("init", "account.settings.relay_urls requires at least one valid ws:// or wss:// relay")
	}
	if settings.WriteQuorum == 0 {
		settings.WriteQuorum = 1
	}
	if settings.WriteQuorum < 1 || settings.WriteQuorum > len(settings.RelayURLs) {
		return AccountSettings{}, invalidArgument("init", "account.settings.write_quorum must be between 1 and the number of relays")
	}
	settings.PublicKey = strings.TrimSpace(settings.PublicKey)
	return settings, nil
}

func normalizeRelayURLs(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" ||
			(parsed.Scheme != "ws" && parsed.Scheme != "wss") {
			return nil, fmt.Errorf("account.settings.relay_urls contains an invalid relay URL")
		}
		normalized := nostrgo.NormalizeURL(parsed.String())
		if normalized != "" && !slices.Contains(result, normalized) {
			result = append(result, normalized)
		}
	}
	return result, nil
}

func (adapter *Adapter) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.ready, adapter.closed = false, true
	return nil
}

var _ socialhub.Adapter = (*Adapter)(nil)
