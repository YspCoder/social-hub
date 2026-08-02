package musicbrainz

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

func testConfig(server *httptest.Server, interval string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": server.URL + "/api", "user_agent": "social-hub-musicbrainz-tests/1.0 (tests@example.com)",
			"request_interval": interval,
		},
		Accounts: []socialhub.AccountConfig{{ID: "public"}, {ID: "secondary"}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, "0s"), socialhub.WithHTTPClient(server.Client())); err != nil {
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
		t.Fatal("MusicBrainz adapter was not registered")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[CapabilityCatalog].Supported || capabilities[CapabilityCatalog].Approval != socialhub.ApprovalGranted ||
		capabilities[socialhub.CapFetch].Supported {
		t.Fatalf("unexpected capabilities: %#v, %v", capabilities, err)
	}
	if client.Platform() != "musicbrainz" || client.Account() != "public" || client.CatalogWorkflow() == nil {
		t.Fatal("unexpected client identity or workflow")
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
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "public"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("expected closed adapter error, got %v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, "0s")); !errors.Is(err, socialhub.ErrConflict) {
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
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "musicbrainz" }},
		{"invalid URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "javascript:bad" }},
		{"path traversal URL", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = server.URL + "/../api" }},
		{"invalid user agent", func(config *socialhub.AdapterConfig) { config.Settings["user_agent"] = " bad " }},
		{"invalid duration", func(config *socialhub.AdapterConfig) { config.Settings["request_interval"] = "bad" }},
		{"negative duration", func(config *socialhub.AdapterConfig) { config.Settings["request_interval"] = "-1s" }},
		{"large duration", func(config *socialhub.AdapterConfig) { config.Settings["request_interval"] = "61s" }},
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
			config := testConfig(server, "0s")
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}

	config := testConfig(server, "0s")
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

func TestRequestGateIsSharedAndCancellable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, `{"count":0,"offset":0,"artists":[]}`)
	}))
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, "30ms"), socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	firstCommon, err := adapter.Client(context.Background(), "public")
	if err != nil {
		t.Fatal(err)
	}
	secondCommon, err := adapter.Client(context.Background(), "secondary")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := firstCommon.(*Client).SearchArtists(context.Background(), SearchRequest{Query: "one"}); err != nil {
		t.Fatal(err)
	}
	if _, err := secondCommon.(*Client).SearchArtists(context.Background(), SearchRequest{Query: "two"}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("shared request gate did not delay the second client: %s", elapsed)
	}

	gate := newRequestGate(time.Second)
	if err := gate.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := gate.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected cancellable wait, got %v", err)
	}
	if err := (*requestGate)(nil).Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}
