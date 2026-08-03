package messenger

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true)
	if adapter.Name() != adapterName || client.Platform() != "facebook" || client.Account() != "main" {
		t.Fatalf("identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	metadata := adapter.Metadata()
	if metadata.Product != productName || metadata.APIVersion != graphVersion || metadata.DocURL != docURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []socialhub.Capability{
		socialhub.CapFetch, socialhub.CapMessage, socialhub.CapWebhook, CapabilityUserProfile, CapabilityMediaMessage,
	} {
		if !capabilities.Has(name) || capabilities[name].DocURL == "" {
			t.Fatalf("capability %s=%#v", name, capabilities[name])
		}
	}
	for _, name := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact} {
		if capabilities.Has(name) {
			t.Fatalf("unsupported capability %s=%#v", name, capabilities[name])
		}
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher should be available")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger should be available")
	}
	webhook, ok := client.WebhookHandler()
	if !ok || webhook == nil {
		t.Fatal("webhook should be available")
	}
	if _, ok := webhook.(socialhub.ChallengeHandler); !ok {
		t.Fatal("challenge handler should be available")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if client.MessageWorkflow() == nil || client.ProfileWorkflow() == nil || client.Close() != nil {
		t.Fatal("typed workflows or close unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, false)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndOptionalWebhook(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false)
	if webhook, ok := client.WebhookHandler(); ok || webhook != nil {
		t.Fatal("webhook should require an app secret")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("webhook capability=%#v", capabilities[socialhub.CapWebhook])
	}

	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{name: "adapter mismatch", mutate: func(c *socialhub.AdapterConfig) { c.Adapter = "wrong" }},
		{name: "missing accounts", mutate: func(c *socialhub.AdapterConfig) { c.Accounts = nil }},
		{name: "missing token", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].AccessTokenRef = "" }},
		{name: "bad Page ID", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["page_id"] = "not-numeric" }},
		{name: "bad endpoint", mutate: func(c *socialhub.AdapterConfig) { c.Settings["base_url"] = "ftp://example.test" }},
		{name: "endpoint credentials", mutate: func(c *socialhub.AdapterConfig) { c.Settings["base_url"] = "https://user@example.test" }},
		{name: "unknown setting", mutate: func(c *socialhub.AdapterConfig) { c.Settings["unknown"] = true }},
		{name: "unknown account setting", mutate: func(c *socialhub.AdapterConfig) { c.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server.URL, false)
			test.mutate(&config)
			adapter := &Adapter{}
			if err := adapter.Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	adapter := &Adapter{}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client before init=%v", err)
	}
	initialized, _ := newTestClient(t, server, false)
	if _, err := initialized.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}

	missingSecret := &Adapter{}
	if err := missingSecret.Init(context.Background(), testConfig(server.URL, false), socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := missingSecret.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token value=%v", err)
	}
}
