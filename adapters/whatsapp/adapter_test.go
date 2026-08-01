package whatsapp

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

var testNow = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type testResolver map[string]string

func (r testResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := r[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

func allScopes() []string {
	return []string{"whatsapp_business_messaging", "whatsapp_business_management"}
}

func testConfig(baseURL string, scopes []string, webhooks bool) socialhub.AdapterConfig {
	settings := map[string]any{}
	if baseURL != "" {
		settings["base_url"] = baseURL
	}
	accountSettings := map[string]any{
		"phone_number_id": "123456789", "business_account_id": "987654321",
	}
	if webhooks {
		accountSettings["app_secret_ref"] = "test://app-secret"
		accountSettings["verify_token_ref"] = "test://verify-token"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Settings: settings,
		Accounts: []socialhub.AccountConfig{{
			ID: "main", AccessTokenRef: "test://access-token",
			Approval: socialhub.ApprovalConfig{Scopes: scopes}, Settings: accountSettings,
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, scopes []string, webhooks bool) *Client {
	t.Helper()
	return newTestClientWithHTTP(t, server.URL, server.Client(), scopes, webhooks)
}

func newTestClientWithHTTP(t *testing.T, baseURL string, httpClient *http.Client, scopes []string, webhooks bool) *Client {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), testConfig(baseURL, scopes, webhooks),
		socialhub.WithHTTPClient(httpClient),
		socialhub.WithSecretResolver(testResolver{
			"test://access-token": "access-token", "test://app-secret": "app-secret",
			"test://verify-token": "verify-token",
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
		t.Errorf("encode response: %v", err)
	}
}

func requireBearer(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer access-token" {
		t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
	}
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, allScopes(), true),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://access-token": "access-token", "test://app-secret": "app-secret", "test://verify-token": "verify-token",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if adapter.Name() != adapterName || client.Platform() != "whatsapp" || client.Account() != "main" {
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
	for _, capability := range []socialhub.Capability{socialhub.CapMessage, CapabilityTypedMessages, CapabilityMedia, CapabilityBusinessProfile} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	if !capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("webhook capability=%#v", capabilities[socialhub.CapWebhook])
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %s=%#v", capability, capabilities[capability])
		}
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger should be enabled")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook should be enabled")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be disabled")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher should be disabled")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common media uploader should be disabled")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be disabled")
	}
	if client.MessageWorkflow() == nil || client.MediaWorkflow() == nil || client.BusinessProfileWorkflow() == nil || client.Close() != nil {
		t.Fatal("typed workflows or close are invalid")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close: %v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, nil, false)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close: %v", err)
	}
}

func TestAdapterValidationSecretsAndScopeGating(t *testing.T) {
	invalid := []socialhub.AdapterConfig{
		{},
		{Adapter: "whatsapp", Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"phone_number_id": "1"}}}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:secret@example.test"}, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"phone_number_id": "1"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", Settings: map[string]any{"phone_number_id": "1"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"phone_number_id": "abc"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"phone_number_id": "1", "business_account_id": "abc"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", AccessTokenRef: "token", Settings: map[string]any{"phone_number_id": "1", "unknown": true}}}},
	}
	for _, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("config=%#v error=%v", config, err)
		}
	}

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, []string{"whatsapp_business_management"}, false),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testResolver{"test://access-token": "access-token"})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing client: %v", err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[socialhub.CapMessage].Approval != socialhub.ApprovalRequired || capabilities[CapabilityBusinessProfile].Approval != socialhub.ApprovalGranted {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be disabled without app secret")
	}
	text := "hello"
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "15550001111", Text: &text}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}

	missing := &Adapter{}
	if err := missing.Init(context.Background(), testConfig(server.URL, nil, true),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := missing.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token error=%v", err)
	}
}
