package qq

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type testResolver map[string]string

func (resolver testResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

var testNow = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

func testConfig(serverURL string, webhook bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{ID: "main", AppID: "102012345", AccessTokenRef: "test://access-token"}
	if webhook {
		account.Webhook.SecretRef = "test://app-secret"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": serverURL, "token_url": serverURL + "/app/getAppAccessToken"},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, webhook bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), testConfig(server.URL, webhook),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://access-token": "access-token", "test://app-secret": "app-secret"}),
		socialhub.WithClock(fixedClock{now: testNow}),
	)
	if err != nil {
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
	adapter, client := newTestClient(t, server, true)
	if adapter.Name() != adapterName || client.Platform() != "qq" || client.Account() != "main" {
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
	for _, capability := range []socialhub.Capability{socialhub.CapMessage, socialhub.CapWebhook, CapabilityTypedMessages, CapabilityURLMedia} {
		if !capabilities.Has(capability) || capabilities[capability].DocURL == "" {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %s=%#v", capability, capabilities[capability])
		}
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("fetcher should be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("common media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger should be available")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook should be available")
	}
	if client.MessageWorkflow() == nil || client.MediaWorkflow() == nil || client.WebhookWorkflow() == nil {
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
	if err := adapter.Init(context.Background(), testConfig(server.URL, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndOptionalWebhook(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	validAccount := socialhub.AccountConfig{ID: "main", AppID: "102012345", AccessTokenRef: "token"}
	invalid := []socialhub.AdapterConfig{
		{Adapter: adapterName},
		{Adapter: "qq", Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Settings: map[string]any{"token_url": "relative"}, Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Settings: map[string]any{"unknown": true}, Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AppID: "app"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AppID: "app", AccessTokenRef: "token", Webhook: socialhub.WebhookConfig{TokenRef: "bad"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AppID: "app", AccessTokenRef: "token", Settings: map[string]any{"unknown": true}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}
	adapter, client := newTestClient(t, server, false)
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("token-only client exposes webhook")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapWebhook) || !strings.Contains(capabilities[socialhub.CapWebhook].Reason, "configure") {
		t.Fatalf("webhook capability=%#v", capabilities[socialhub.CapWebhook])
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
}

func TestClientCredentialResolution(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	config := testConfig(server.URL, false)
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token=%v", err)
	}

	config.Accounts[0].Webhook.SecretRef = "test://app-secret"
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithSecretResolver(testResolver{"test://access-token": "token", "test://app-secret": strings.Repeat("x", 513)}),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oversized AppSecret=%v", err)
	}
}
