package unitypublisher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterRegistrationMetadataCapabilitiesAndLifecycle(t *testing.T) {
	found := false
	for _, name := range socialhub.Adapters() {
		if name == adapterName {
			found = true
		}
	}
	if !found {
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	adapter, client := newTestAdapter(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL == "" || metadata.VerifiedAt.IsZero() || contractVersion != "1.0.0" {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityPublisherManagement].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities[capability].Supported {
			t.Fatalf("organic capability %q unexpectedly supported", capability)
		}
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher unexpectedly supported")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("Fetcher unexpectedly supported")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader unexpectedly supported")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("Reactor unexpectedly supported")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger unexpectedly supported")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("WebhookHandler unexpectedly supported")
	}
	if client.Applications() == nil || client.Placements() == nil || client.TestDevices() == nil ||
		client.Platform() != platformName || client.Account() != testAccountID || client.Close() != nil {
		t.Fatal("typed workflows or client identity are invalid")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("closed client error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("re-init error=%v", err)
	}
}

func TestAdapterConfigurationValidation(t *testing.T) {
	base := testConfig("https://example.test")
	tests := []struct {
		name string
		edit func(*socialhub.AdapterConfig)
	}{
		{"missing account", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"product mismatch", func(config *socialhub.AdapterConfig) { config.Product = "other" }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"bad base URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.test" }},
		{"base query", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://example.test?x=1" }},
		{"base trailing slash", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://example.test/" }},
		{"bad organization", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["organization_id"] = "0" }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"missing credentials", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"partial basic", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].AccessTokenRef, config.Accounts[0].ClientID = "", testKeyID
		}},
		{"bad basic key ID", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].AccessTokenRef, config.Accounts[0].ClientID, config.Accounts[0].SecretRef = "", "bad:key", "secret://key"
		}},
		{"both credentials", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].ClientID, config.Accounts[0].SecretRef = testKeyID, "secret://key"
		}},
		{"static with secret", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "secret://key" }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.SecretRef = "secret://webhook" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Settings = cloneMap(base.Settings)
			config.Accounts = append([]socialhub.AccountConfig(nil), base.Accounts...)
			config.Accounts[0].Settings = cloneMap(base.Accounts[0].Settings)
			test.edit(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestBasicAndBearerClientAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != testKeyID || password != testSecretKey || request.Header.Get("Accept") != "application/json" {
			t.Fatalf("basic auth=%q/%q ok=%t headers=%v", username, password, ok, request.Header)
		}
		writeJSON(t, writer, http.StatusOK, []Application{applicationFixture()})
	}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), basicConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://unity-key": testSecretKey}),
	); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.Client(context.Background(), testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := value.(*Client).ListApplications(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterClientAndSecretFailures(t *testing.T) {
	adapter := &Adapter{}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client error=%v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	if err := adapter.Init(context.Background(), testConfig(server.URL), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("secret resolution error=%v", err)
	}
	if _, err := resolveSecret(context.Background(), mapResolver{}, "missing", "test"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing secret error=%v", err)
	}
	client := cloneHTTPClient(http.DefaultClient)
	if client == http.DefaultClient || client.CheckRedirect == nil {
		t.Fatal("HTTP client was not cloned with redirect rejection")
	}
}
