package mastodon

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

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	config := socialhub.AdapterConfig{
		Adapter: adapterName,
		Product: "mastodon-rest-api",
		Accounts: []socialhub.AccountConfig{{
			ID: "fediverse-main", ClientID: "client-id", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{"instance_url": server.URL, "user_id": "account-1"},
			Approval: socialhub.ApprovalConfig{Scopes: scopes},
		}},
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret", "test://access-token": "access-token"}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "fediverse-main")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func allTestScopes() []string {
	return []string{"read:accounts", "read:statuses", "write:statuses", "write:media", "write:favourites"}
}

func TestAdapterRegistrationAndCapabilities(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, allTestScopes())

	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact,
		CapabilityHomeTimeline, CapabilityInstanceDiscovery,
	} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s unavailable: %#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMessage) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("unsupported capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher should be available")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher should be available")
	}
	if _, ok := client.MediaUploader(); !ok {
		t.Fatal("media uploader should be available")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor should be available")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook handler should be unavailable")
	}
	if client.TimelineWorkflow() == nil || client.InstanceWorkflow() == nil {
		t.Fatal("typed workflows should be available")
	}
	metadata := adapter.Metadata()
	if metadata.APIVersion != apiVersion || metadata.Product != "mastodon-rest-api" {
		t.Fatalf("metadata=%#v", metadata)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "fediverse-main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
}

func TestAdapterValidationAndScopeGating(t *testing.T) {
	invalid := []socialhub.AdapterConfig{
		{Adapter: "mastodon/other", Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "test://token", Settings: map[string]any{"instance_url": "https://example.test"}}}},
		{Adapter: adapterName, Settings: map[string]any{"instance_url": "https://example.test"}, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "test://token", Settings: map[string]any{"instance_url": "https://example.test"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "test://token", Settings: map[string]any{"instance_url": "https://example.test/path"}}}},
	}
	for _, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("config=%#v error=%v", config, err)
		}
	}

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, []string{"read"})
	text := "blocked"
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("publish scope error=%v", err)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || capabilities.Has(socialhub.CapPublish) || !capabilities.Has(socialhub.CapFetch) {
		t.Fatalf("capabilities=%#v error=%v", capabilities, err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
}

func TestScopeInheritance(t *testing.T) {
	if !scopeGranted([]string{"read"}, "read:accounts") || !scopeGranted([]string{"write"}, "write:media") {
		t.Fatal("parent scopes should grant child scopes")
	}
	if scopeGranted([]string{"read:accounts"}, "read:statuses") || scopeGranted([]string{"write:media"}, "write:statuses") {
		t.Fatal("unrelated scopes should not be granted")
	}
}
