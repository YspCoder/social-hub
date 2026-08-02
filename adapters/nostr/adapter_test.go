package nostr

import (
	"context"
	"errors"
	"testing"

	nostrgo "fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"

	"social-hub/pkg/socialhub"
)

func TestAdapterReadOnlyAndWritableAccounts(t *testing.T) {
	secret := nostrgo.KeyOne
	fixture := newRelayFixture(t)

	_, readOnly := openTestClient(t, []string{fixture.url}, nip19.EncodeNprofile(secret.Public(), []string{fixture.url}), "", 1)
	if _, ok := readOnly.Publisher(); ok {
		t.Fatal("read-only client unexpectedly exposes Publisher")
	}
	if _, ok := readOnly.Reactor(); ok {
		t.Fatal("read-only client unexpectedly exposes Reactor")
	}
	capabilities, err := readOnly.Capabilities(context.Background())
	if err != nil || capabilities[socialhub.CapFetch].Supported != true || capabilities[socialhub.CapPublish].Supported {
		t.Fatalf("unexpected read-only capabilities: %#v, %v", capabilities, err)
	}

	_, writable := openTestClient(t, []string{fixture.url}, nip19.EncodeNpub(secret.Public()), nip19.EncodeNsec(secret), 1)
	if publisher, ok := writable.Publisher(); !ok || publisher == nil {
		t.Fatal("writable client does not expose Publisher")
	}
	if workflow, ok := writable.InteractionWorkflow(); !ok || workflow == nil {
		t.Fatal("writable client does not expose InteractionWorkflow")
	}
	if writable.publicKey != secret.Public() {
		t.Fatalf("public key mismatch: %s", writable.publicKey.Hex())
	}
}

func TestAdapterValidationAndLifecycle(t *testing.T) {
	validPublicKey := nostrgo.KeyOne.Public().Hex()
	tests := []struct {
		name    string
		config  socialhub.AdapterConfig
		wantErr error
	}{
		{name: "wrong adapter", config: accountConfig("wrong", []string{"wss://relay.example"}, validPublicKey, 1), wantErr: socialhub.ErrInvalidArgument},
		{name: "missing identity", config: accountConfig(adapterName, []string{"wss://relay.example"}, "", 1), wantErr: socialhub.ErrInvalidArgument},
		{name: "http relay", config: accountConfig(adapterName, []string{"https://relay.example"}, validPublicKey, 1), wantErr: socialhub.ErrInvalidArgument},
		{name: "relay credentials", config: accountConfig(adapterName, []string{"wss://user:pass@relay.example"}, validPublicKey, 1), wantErr: socialhub.ErrInvalidArgument},
		{name: "relay query", config: accountConfig(adapterName, []string{"wss://relay.example?token=x"}, validPublicKey, 1), wantErr: socialhub.ErrInvalidArgument},
		{name: "bad public key", config: accountConfig(adapterName, []string{"wss://relay.example"}, "npub1bad", 1), wantErr: socialhub.ErrInvalidArgument},
		{name: "bad quorum", config: accountConfig(adapterName, []string{"wss://relay.example"}, validPublicKey, 2), wantErr: socialhub.ErrInvalidArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := &Adapter{}
			err := adapter.Init(context.Background(), test.config)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Init() error = %v, want %v", err, test.wantErr)
			}
		})
	}

	config := accountConfig(adapterName, []string{"wss://relay.example", "wss://relay.example/"}, validPublicKey, 1)
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if adapter.Metadata().APIVersion != apiVersion || adapter.Name() != adapterName {
		t.Fatalf("unexpected metadata: %#v", adapter.Metadata())
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error = %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed adapter client error = %v", err)
	}
	if err := adapter.Init(context.Background(), config); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed adapter init error = %v", err)
	}
}

func TestAdapterRejectsCredentialMismatchAndInvalidSecret(t *testing.T) {
	fixture := newRelayFixture(t)
	other := nostrgo.Generate()
	config := accountConfig(adapterName, []string{fixture.url}, other.Public().Hex(), 1)
	config.Accounts[0].AccessTokenRef = "secret://nostr"
	adapter := &Adapter{}
	resolver := socialhub.WithSecretResolver(secretMap{"secret://nostr": nostrgo.KeyOne.Hex()})
	if err := adapter.Init(context.Background(), config, resolver); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("mismatched key error = %v", err)
	}

	config.Accounts[0].Settings["public_key"] = ""
	badAdapter := &Adapter{}
	if err := badAdapter.Init(context.Background(), config, socialhub.WithSecretResolver(secretMap{"secret://nostr": "not-a-secret"})); err != nil {
		t.Fatalf("Init with deferred secret error: %v", err)
	}
	if _, err := badAdapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("invalid secret error = %v", err)
	}
}

func TestRegistration(t *testing.T) {
	registered := socialhub.Adapters()
	found := false
	for _, name := range registered {
		found = found || name == adapterName
	}
	if !found {
		t.Fatalf("%s is not registered", adapterName)
	}
	config := accountConfig(adapterName, []string{"wss://relay.example"}, nostrgo.KeyOne.Public().Hex(), 1)
	adapter, err := socialhub.Open(context.Background(), adapterName, config)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	_ = adapter.Close()
}

func accountConfig(name string, relays []string, publicKey string, quorum int) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: name,
		Accounts: []socialhub.AccountConfig{{
			ID:       "primary",
			Settings: map[string]any{"relay_urls": relays, "public_key": publicKey, "write_quorum": quorum},
		}},
	}
}
