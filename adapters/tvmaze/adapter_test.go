package tvmaze

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"social-hub/pkg/socialhub"
)

func testConfig(server *httptest.Server) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": server.URL + "/api", "user_agent": "social-hub-tvmaze-tests/1.0",
		},
		Accounts: []socialhub.AccountConfig{{ID: "public"}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server), socialhub.WithHTTPClient(server.Client())); err != nil {
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
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatal("TVmaze adapter was not registered")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityCatalog].Supported || capabilities[CapabilitySchedule].Approval != socialhub.ApprovalGranted ||
		!capabilities[CapabilityPeople].Supported || !capabilities[CapabilityUpdates].Supported || capabilities[socialhub.CapFetch].Supported {
		t.Fatalf("unexpected capabilities: %#v, %v", capabilities, err)
	}
	if client.Platform() != "tvmaze" || client.Account() != "public" {
		t.Fatal("unexpected client identity")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher unexpectedly exposed")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher unexpectedly exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader unexpectedly exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor unexpectedly exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger unexpectedly exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook unexpectedly exposed")
	}
	if client.CatalogWorkflow() == nil || client.ScheduleWorkflow() == nil || client.PeopleWorkflow() == nil || client.UpdatesWorkflow() == nil {
		t.Fatal("typed workflows are missing")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "public"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed adapter error, got %v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed init error, got %v", err)
	}
}

func TestConfigurationAndClientValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	if _, err := (&Adapter{}).Client(context.Background(), "public"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected uninitialized adapter error, got %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"missing accounts", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "tvmaze" }},
		{"invalid URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "javascript:bad" }},
		{"path traversal URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = server.URL + "/../api" }},
		{"invalid user agent", func(config *socialhub.AdapterConfig) { config.Settings["user_agent"] = " bad " }},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Settings["unknown"] = true }},
		{"client ID", func(config *socialhub.AdapterConfig) { config.Accounts[0].ClientID = "client" }},
		{"app ID", func(config *socialhub.AdapterConfig) { config.Accounts[0].AppID = "app" }},
		{"secret", func(config *socialhub.AdapterConfig) { config.Accounts[0].SecretRef = "env://SECRET" }},
		{"access token", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "env://TOKEN" }},
		{"token store", func(config *socialhub.AdapterConfig) { config.Accounts[0].TokenStore = "tokens" }},
		{"webhook", func(config *socialhub.AdapterConfig) { config.Accounts[0].Webhook.TokenRef = "env://TOKEN" }},
		{"approval", func(config *socialhub.AdapterConfig) { config.Accounts[0].Approval.Scopes = []string{"read"} }},
		{"account settings", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings = map[string]any{"x": true} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}

	config := testConfig(server)
	if err := (&Adapter{}).Init(context.Background(), config, socialhub.WithHTTPClient(nil)); err == nil {
		t.Fatal("expected invalid adapter option")
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("expected missing account error, got %v", err)
	}
	if _, err := adapter.Client(context.Background(), "public", socialhub.WithHTTPClient(nil)); err == nil {
		t.Fatal("expected invalid client option")
	}
}
