package telegram

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

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, ok := r[reference]
	if !ok {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newTestAdapter(t *testing.T, server *httptest.Server, defaultChatID string, withWebhook bool) (*Adapter, *Client) {
	t.Helper()
	account := socialhub.AccountConfig{
		ID:             "primary",
		AccessTokenRef: "test://bot-token",
		Settings:       map[string]any{"default_chat_id": defaultChatID},
	}
	if withWebhook {
		account.Webhook.SecretRef = "test://webhook-secret"
	}
	config := socialhub.AdapterConfig{
		Adapter:  adapterName,
		Product:  "bot-api",
		Settings: map[string]any{"base_url": server.URL},
		Accounts: []socialhub.AccountConfig{account},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://bot-token":      "123456:bot-token",
			"test://webhook-secret": "webhook_secret-1",
		}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationAndCapabilitySurface(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, "@news", true)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMessage, socialhub.CapWebhook, CapabilityMediaSend} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	if publisher, ok := client.Publisher(); !ok || publisher == nil {
		t.Fatal("publisher unavailable")
	}
	if messenger, ok := client.Messenger(); !ok || messenger == nil {
		t.Fatal("messenger unavailable")
	}
	if webhook, ok := client.WebhookHandler(); !ok || webhook == nil {
		t.Fatal("webhook unavailable")
	}
	if client.BotWorkflow() == nil {
		t.Fatal("bot workflow unavailable")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher should be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestOptionalCapabilitiesFollowAccountConfiguration(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "", false)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capabilities.Has(socialhub.CapPublish) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should require default_chat_id")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should require secret_ref")
	}
}

func TestAdapterRequiresBotTokenReference(t *testing.T) {
	config := socialhub.AdapterConfig{Adapter: adapterName, Product: "bot-api", Accounts: []socialhub.AccountConfig{{ID: "primary"}}}
	if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}
