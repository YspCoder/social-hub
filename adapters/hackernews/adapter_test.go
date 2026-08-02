package hackernews

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

var testNow = time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)

func testConfig(server *httptest.Server) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": server.URL + "/api", "user_agent": "social-hub-hackernews-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "public", Settings: map[string]any{"default_feed": "topstories"},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "public")
	if err != nil {
		t.Fatal(err)
	}
	client, ok := common.(*Client)
	if !ok {
		t.Fatalf("unexpected client type %T", common)
	}
	return adapter, client
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func TestAdapterLifecycleRegistrationAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server)

	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Name != adapterName || metadata.Product != productName ||
		metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatal("Hacker News adapter was not registered")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapFetch, CapabilityItems, CapabilityFeeds, CapabilityUsers, CapabilityUpdates,
	} {
		if !capabilities.Has(capability) || capabilities[capability].DocURL != documentationURL {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %s=%#v", capability, capabilities[capability])
		}
	}
	if fetcher, ok := client.Fetcher(); !ok || fetcher == nil {
		t.Fatal("fetcher should be available")
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
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if client.ItemWorkflow() == nil || client.FeedWorkflow() == nil || client.UserWorkflow() == nil || client.UpdatesWorkflow() == nil {
		t.Fatal("typed workflows should be available")
	}
	if client.Platform() != "hackernews" || client.Account() != "public" {
		t.Fatal("client identity is invalid")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "public"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestConfigurationAndClientValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if _, err := (&Adapter{}).Client(context.Background(), "public"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized adapter=%v", err)
	}
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"missing accounts", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "hackernews" }},
		{"invalid URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "javascript:bad" }},
		{"URL credentials", func(config *socialhub.AdapterConfig) {
			config.Settings["base_url"] = "https://user:pass@example.test/v0"
		}},
		{"path traversal", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = server.URL + "/../v0" }},
		{"invalid user agent", func(config *socialhub.AdapterConfig) { config.Settings["user_agent"] = " bad " }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"client ID", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "client" }},
		{"app ID", func(config *socialhub.AdapterConfig) { config.Accounts[0].AppID = "app" }},
		{"secret", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "env://SECRET" }},
		{"access token", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "env://TOKEN" }},
		{"token store", func(config *socialhub.AdapterConfig) { config.Accounts[0].TokenStore = "tokens" }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.TokenRef = "env://TOKEN" }},
		{"approval", func(config *socialhub.AdapterConfig) { config.Accounts[0].Approval.Scopes = []string{"read"} }},
		{"invalid feed", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["default_feed"] = "front" }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	config := testConfig(server)
	if err := (&Adapter{}).Init(context.Background(), config, socialhub.WithHTTPClient(nil)); err == nil {
		t.Fatal("nil HTTP client accepted")
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "public", socialhub.WithHTTPClient(nil)); err == nil {
		t.Fatal("nil client HTTP option accepted")
	}
	config = testConfig(server)
	config.Accounts[0].Settings = nil
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "public")
	if err != nil || common.(*Client).defaultFeed != FeedTop {
		t.Fatalf("default feed client=%#v err=%v", common, err)
	}
}
