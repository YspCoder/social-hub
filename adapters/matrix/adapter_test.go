package matrix

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

var testNow = time.Date(2026, time.August, 2, 8, 0, 0, 0, time.UTC)

type testSecrets map[string]string

func (secrets testSecrets) Resolve(_ context.Context, reference string) (string, error) {
	value, found := secrets[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(homeserverURL string, defaultRoom bool) socialhub.AdapterConfig {
	settings := map[string]any{
		"homeserver_url": homeserverURL,
		"user_id":        "@hub:example.test",
		"device_id":      "DEVICE-1",
	}
	if defaultRoom {
		settings["default_room_id"] = "!room/alpha:example.test"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Accounts: []socialhub.AccountConfig{{
			ID: "main", AccessTokenRef: "secret://matrix-token", Settings: settings,
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, defaultRoom bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(
		context.Background(), testConfig(server.URL, defaultRoom),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testSecrets{"secret://matrix-token": "matrix-token"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatalf("Init: %v", err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	client, ok := common.(*Client)
	if !ok {
		t.Fatalf("client type %T", common)
	}
	return adapter, client
}

func TestAdapterLifecycleRegistrationAndCapabilities(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true)

	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter %q is not registered", adapterName)
	}
	if client.Platform() != "matrix" || client.Account() != "main" || client.EventWorkflow() == nil || client.MediaWorkflow() == nil || client.SyncWorkflow() == nil {
		t.Fatal("client identity or typed workflows are unavailable")
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapReact, socialhub.CapMessage, CapabilityEvents, CapabilityMedia, CapabilitySync} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s unavailable: %#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMedia) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("unsupported common capabilities=%#v", capabilities)
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
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common media uploader must be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler must be unavailable")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close error=%v", err)
	}
}

func TestAdapterWithoutDefaultRoom(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || capabilities.Has(socialhub.CapPublish) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher must require a default room")
	}
	text := "hello"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("publish error=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("list posts error=%v", err)
	}
}

func TestAdapterValidationAndClientErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "matrix/other" }},
		{"global settings", func(config *socialhub.AdapterConfig) { config.Settings = map[string]any{"homeserver_url": server.URL} }},
		{"homeserver path", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["homeserver_url"] = server.URL + "/client"
		}},
		{"user", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_id"] = "alice" }},
		{"device", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["device_id"] = "bad\nvalue" }},
		{"room", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["default_room_id"] = "room" }},
		{"token", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"unknown", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig(server.URL, true)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	if err := (&Adapter{}).Init(context.Background(), socialhub.AdapterConfig{Adapter: adapterName}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("base config error=%v", err)
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, true), socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testSecrets{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing secret error=%v", err)
	}
}

var _ socialhub.SecretResolver = testSecrets{}
var _ socialhub.Clock = fixedClock{}
