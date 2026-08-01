package discord

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

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

func newTestClient(t *testing.T, server *httptest.Server, defaultChannelID string) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName, Product: "bot-api",
		Settings: map[string]any{"base_url": server.URL, "cdn_url": server.URL + "/cdn", "user_agent": "DiscordBot (test, 1)"},
		Accounts: []socialhub.AccountConfig{{
			ID: "primary", AccessTokenRef: "test://bot-token",
			Settings: map[string]any{"default_channel_id": defaultChannelID},
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://bot-token": "bot-token"}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "primary")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationAndCapabilities(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, "100")
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapReact, socialhub.CapMessage, CapabilityGateway} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %q=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMedia) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger unavailable")
	}
	if client.BotWorkflow() == nil {
		t.Fatal("bot workflow unavailable")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "primary"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestPublisherRequiresDefaultChannel(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, "")
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capabilities.Has(socialhub.CapPublish) {
		t.Fatalf("publish capability=%#v", capabilities[socialhub.CapPublish])
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
}

func TestAdapterRejectsInvalidAccountConfiguration(t *testing.T) {
	config := socialhub.AdapterConfig{
		Adapter:  adapterName,
		Accounts: []socialhub.AccountConfig{{ID: "primary", AccessTokenRef: "test://token", Settings: map[string]any{"default_channel_id": "not-a-snowflake"}}},
	}
	if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("error=%v", err)
	}
}
