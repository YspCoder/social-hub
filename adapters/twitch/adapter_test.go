package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

var testNow = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type testResolver map[string]string

func (r testResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func testConfig(baseURL string, scopes []string) socialhub.AdapterConfig {
	settings := map[string]any{}
	if baseURL != "" {
		settings["base_url"] = baseURL
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Settings: settings,
		Accounts: []socialhub.AccountConfig{{
			ID: "main", ClientID: "twitch-client", SecretRef: "test://client-secret", AccessTokenRef: "test://user-token",
			Approval: socialhub.ApprovalConfig{Scopes: scopes},
			Settings: map[string]any{
				"user_id": "user-1", "app_access_token_ref": "test://app-token",
				"eventsub_secret_ref": "test://eventsub-secret",
			},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, scopes []string) *Client {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), testConfig(server.URL, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://client-secret": "client-secret", "test://user-token": "user-token",
			"test://app-token": "app-token", "test://eventsub-secret": "eventsub-secret-123",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	)
	if err != nil {
		t.Fatalf("init adapter: %v", err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client, ok := common.(*Client)
	if !ok {
		t.Fatalf("unexpected client type %T", common)
	}
	return client
}

func writeTestJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func TestAdapterLifecycleAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, []string{"user:write:chat", "clips:edit"})
	if client.Platform() != "twitch" || client.Account() != "main" {
		t.Fatalf("unexpected identity: %s %s", client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapMessage) || !capabilities.Has(socialhub.CapWebhook) || !capabilities.Has(CapabilityLive) || !capabilities.Has(CapabilityEventSub) {
		t.Fatalf("unexpected capabilities: %#v err=%v", capabilities, err)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must be disabled")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher must be enabled")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger must be enabled")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook must be enabled")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader must be disabled")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor must be disabled")
	}
	if client.LiveWorkflow() == nil || client.EventSubWorkflow() == nil || client.Close() != nil {
		t.Fatal("typed workflows or close are invalid")
	}
}

func TestAdapterRegistrationMetadataAndOAuth(t *testing.T) {
	if !contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	registered := &Adapter{}
	metadata := registered.Metadata()
	if metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL == "" || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata: %#v", metadata)
	}
	adapter := registered
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if err := adapter.Init(context.Background(), testConfig(server.URL, nil),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://client-secret": "client-secret", "test://user-token": "user-token",
			"test://app-token": "app-token", "test://eventsub-secret": "eventsub-secret-123",
		}), socialhub.WithClock(fixedClock{now: testNow})); err != nil {
		t.Fatalf("init: %v", err)
	}
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil || oauth.ClientID != "twitch-client" || oauth.ClientSecret != "client-secret" {
		t.Fatalf("oauth: %#v %v", oauth, err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing oauth: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client: %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close: %v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close: %v", err)
	}
}

func TestAdapterValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	cases := []socialhub.AdapterConfig{
		{},
		{Adapter: "wrong", Accounts: []socialhub.AccountConfig{{ID: "one"}}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{{ID: "one", ClientID: "id", AccessTokenRef: "token"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", ClientID: "id"}}},
	}
	for index, config := range cases {
		if err := (&Adapter{}).Init(context.Background(), config); err == nil {
			t.Fatalf("case %d accepted invalid config", index)
		}
	}
	config := testConfig(server.URL, nil)
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://user-token": "user-token", "test://client-secret": "client-secret", "test://app-token": "app-token", "test://eventsub-secret": "short"}),
		socialhub.WithClock(fixedClock{now: testNow})); err != nil {
		t.Fatalf("init should defer secret validation: %v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid EventSub secret: %v", err)
	}
	if validEndpoint("ftp://example.test") || validEndpoint("https://user:pass@example.test") || !validEndpoint(server.URL) {
		t.Fatal("endpoint validation mismatch")
	}
}
