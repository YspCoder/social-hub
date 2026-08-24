package viber

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type testResolver map[string]string

func (r testResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)

func testConfig(serverURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter:  adapterName,
		Settings: map[string]any{"base_url": serverURL},
		Accounts: []socialhub.AccountConfig{{
			ID: "main", AccessTokenRef: "test://bot-token",
			Settings: map[string]any{"sender_name": "Social Hub", "sender_avatar": "https://cdn.example/avatar.jpg"},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://bot-token": "bot-token"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func writeTestJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server)
	if adapter.Name() != adapterName || client.Platform() != "viber" || client.Account() != "main" {
		t.Fatalf("identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	metadata := adapter.Metadata()
	if metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != docURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []socialhub.Capability{
		socialhub.CapFetch, socialhub.CapMessage, socialhub.CapWebhook, CapabilityTypedMessages,
		CapabilityBroadcast, CapabilityProfiles, CapabilityPresence, CapabilityWebhookManagement,
	} {
		if !capabilities.Has(name) || capabilities[name].Capability != name || capabilities[name].DocURL == "" {
			t.Fatalf("capability %s=%#v", name, capabilities[name])
		}
	}
	for _, name := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact} {
		if capabilities.Has(name) {
			t.Fatalf("unsupported capability %s=%#v", name, capabilities[name])
		}
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher should be available")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger should be available")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook handler should be available")
	}
	if client.MessageWorkflow() == nil || client.AccountWorkflow() == nil || client.WebhookWorkflow() == nil {
		t.Fatal("typed workflows should be available")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndMissingAccount(t *testing.T) {
	invalid := []socialhub.AdapterConfig{
		{Adapter: "viber", Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"sender_name": "Bot"}}}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"sender_name": "Bot"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", Settings: map[string]any{"sender_name": "Bot"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"sender_name": ""}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"sender_name": "Bot", "sender_avatar": "ftp://example.test/a.jpg"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"sender_name": "Bot", "unknown": true}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, _ := newTestClient(t, server)
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if err := (&Adapter{}).Init(context.Background(), testConfig(server.URL), socialhub.WithSecretResolver(nil)); err == nil {
		t.Fatal("nil secret resolver should fail")
	}
}
