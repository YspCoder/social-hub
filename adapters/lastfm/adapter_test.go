package lastfm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

const (
	testAPIKey    = "0123456789abcdef0123456789abcdef"
	testAPISecret = "api-secret"
	testSession   = "session-key"
)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := resolver[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func testConfig(server *httptest.Server, credentials bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "listener", ClientID: testAPIKey, Settings: map[string]any{"username": "test-user"},
	}
	if credentials {
		account.SecretRef = "test://secret"
		account.AccessTokenRef = "test://session"
	}
	return socialhub.AdapterConfig{
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": server.URL + "/2.0/", "auth_url": server.URL + "/api/auth/"},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, credentials bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, credentials),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://secret": testAPISecret, "test://session": testSession}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "listener")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []socialhub.Capability{CapabilityAuth, CapabilityDiscovery, CapabilityUser, CapabilityListening, CapabilityLibrary} {
		if !capabilities.Has(name) {
			t.Fatalf("capability %s=%#v", name, capabilities[name])
		}
	}
	for _, name := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(name) {
			t.Fatalf("common capability %s must be unsupported", name)
		}
	}
	if client.Platform() != "lastfm" || client.Account() != "listener" || client.Close() != nil ||
		client.AuthWorkflow() == nil || client.DiscoveryWorkflow() == nil || client.UserWorkflow() == nil ||
		client.ListeningWorkflow() == nil || client.LibraryWorkflow() == nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must not be exposed")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "listener"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationCredentialsAndPublicClient(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server, false)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "ftp://example.test" }},
		{"auth URL query", func(config *socialhub.AdapterConfig) { config.Settings["auth_url"] = "https://example.test/auth?x=1" }},
		{"API key length", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "short" }},
		{"API key alphabet", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].ClientID = "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
		}},
		{"username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["username"] = "bad\nname" }},
		{"unknown adapter setting", func(config *socialhub.AdapterConfig) { config.Settings["other"] = true }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["other"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(valid)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	adapter, publicClient := newTestClient(t, server, false)
	capabilities, _ := publicClient.Capabilities(context.Background())
	for _, name := range []socialhub.Capability{CapabilityAuth, CapabilityListening, CapabilityLibrary} {
		if capabilities.Has(name) || capabilities[name].Approval != socialhub.ApprovalRequired {
			t.Fatalf("credential-gated capability %s=%#v", name, capabilities[name])
		}
	}
	if _, err := publicClient.RequestToken(context.Background()); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("auth gate=%v", err)
	}
	if err := publicClient.LoveTrack(context.Background(), TrackRef{Artist: "A", Track: "T"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("write gate=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}

	broken := &Adapter{}
	config := testConfig(server, true)
	if err := broken.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := broken.Client(context.Background(), "listener"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("unresolved secret=%v", err)
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = cloneMap(input.Settings)
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	output.Accounts[0].Settings = cloneMap(input.Accounts[0].Settings)
	return output
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
