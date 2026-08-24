package taboola

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL == "" || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityCampaignManagement].Supported || !capabilities[CapabilityReporting].Supported {
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
	if client.Accounts() == nil || client.Campaigns() == nil || client.Items() == nil || client.Reports() == nil || client.Platform() != platformName || client.Account() != testAccountID || client.Close() != nil {
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
		{"token trailing slash", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = "https://example.test/token/" }},
		{"bad advertiser", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["advertiser_account_id"] = "bad/id" }},
		{"missing credentials", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"both credentials", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].ClientID, config.Accounts[0].SecretRef = "id", "secret://client"
		}},
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

func TestAdapterClientAndOAuthValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	adapter := &Adapter{}
	if _, err := adapter.Client(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized OAuth error=%v", err)
	}
	adapter, _ = newTestAdapter(t, server)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), testAccountID); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("static-token OAuth error=%v", err)
	}

	config := testConfig(server.URL)
	config.Accounts[0].AccessTokenRef = ""
	config.Accounts[0].ClientID = "client-id"
	config.Accounts[0].SecretRef = "secret://client"
	credentials := &Adapter{}
	if err := credentials.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"secret://client": "client-secret"}),
		socialhub.WithClock(fixedClock{value: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	oauth, err := credentials.OAuth(context.Background(), testAccountID)
	if err != nil || oauth.ClientID != "client-id" || oauth.ClientSecret != "client-secret" || strings.HasSuffix(oauth.TokenURL, "/") {
		t.Fatalf("OAuth=%#v err=%v", oauth, err)
	}
}

func TestSecretResolutionAndEndpointHelpers(t *testing.T) {
	if _, err := resolveSecret(context.Background(), mapResolver{}, "missing", "test"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing secret error=%v", err)
	}
	if validEndpoint("https://user:pass@example.test") || validEndpoint("mailto:test@example.test") || !validEndpoint("https://example.test/path") {
		t.Fatal("endpoint validation mismatch")
	}
	client := cloneHTTPClient(http.DefaultClient)
	if client == http.DefaultClient || client.CheckRedirect == nil {
		t.Fatal("HTTP client was not cloned with redirect rejection")
	}
}

func cloneMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
