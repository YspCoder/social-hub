package threads

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
	value, found := r[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var testNow = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

func allScopes() []string {
	return []string{
		"threads_basic", "threads_content_publish", "threads_manage_insights", "threads_manage_replies",
		"threads_read_replies", "threads_keyword_search", "threads_manage_mentions", "threads_delete",
		"threads_location_tagging", "threads_profile_discovery",
	}
}

func testConfig(serverURL string, scopes []string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": serverURL, "auth_url": serverURL + "/oauth/authorize", "token_url": serverURL + "/oauth/access_token",
			"long_token_url": serverURL + "/access_token", "refresh_url": serverURL + "/refresh_access_token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "main", ClientID: "threads-app-id", SecretRef: "test://app-secret", AccessTokenRef: "test://access-token",
			Settings: map[string]any{"user_id": "user-1"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://app-secret": "app-secret", "test://access-token": "access-token"}),
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
		t.Errorf("encode test response: %v", err)
	}
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, allScopes())
	if adapter.Name() != adapterName || client.Platform() != "threads" || client.Account() != "main" {
		t.Fatalf("adapter/client identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapReact, CapabilityContainerPublish,
		CapabilityInsights, CapabilityDiscovery, CapabilityReplyModeration, CapabilityRepost, CapabilityPublishingQuota,
	} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMedia) || capabilities.Has(socialhub.CapMessage) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("unsupported capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher should be available")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher should be available")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor should be available")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common media uploader should be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if client.ContainerWorkflow() == nil || client.InsightsWorkflow() == nil || client.DiscoveryWorkflow() == nil || client.ModerationWorkflow() == nil || client.RepostWorkflow() == nil || client.PublishingQuotaWorkflow() == nil {
		t.Fatal("typed workflows should be available")
	}
	metadata := adapter.Metadata()
	if metadata.APIVersion != apiVersion || metadata.Product != productName || metadata.DocURL != docURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil || oauth.ClientID != "threads-app-id" || oauth.ClientSecret != "app-secret" || oauth.Clock == nil {
		t.Fatalf("oauth=%#v error=%v", oauth, err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, allScopes())); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinitialize closed adapter error=%v", err)
	}
}

func TestAdapterValidationAndScopeGating(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	invalid := []socialhub.AdapterConfig{
		{Adapter: "threads", Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"user_id": "1"}}}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:secret@example.test"}, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"user_id": "1"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", Settings: map[string]any{"user_id": "1"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"user_id": "1", "unknown": true}}}},
	}
	for _, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("config=%#v error=%v", config, err)
		}
	}

	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, []string{"threads_basic"}),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://app-secret": "app-secret", "test://access-token": "access-token"}),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth account error=%v", err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[socialhub.CapPublish].Approval != socialhub.ApprovalRequired || capabilities[socialhub.CapFetch].Approval != socialhub.ApprovalRequired {
		t.Fatalf("gated capabilities=%#v", capabilities)
	}
	if err := client.requireScope("publish", "threads_content_publish", "threads_delete"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
}
