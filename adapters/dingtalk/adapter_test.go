package dingtalk

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

var testNow = time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)

func testConfig(serverURL string, managed, robot bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "test://access-token",
		Approval: socialhub.ApprovalConfig{Scopes: []string{"Contact.User.Read"}},
		Settings: map[string]any{"corp_id": "corp-1"},
	}
	if managed {
		account.AccessTokenRef = ""
		account.ClientID = "ding-client"
		account.SecretRef = "test://client-secret"
	}
	if robot {
		account.Settings["robot_code"] = "ding-robot"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": serverURL}, Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, managed, robot bool, options ...socialhub.Option) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	base := []socialhub.Option{
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://access-token": "access-token", "test://client-secret": "client-secret",
		}),
		socialhub.WithClock(fixedClock{now: testNow}),
	}
	base = append(base, options...)
	if err := adapter.Init(context.Background(), testConfig(server.URL, managed, robot), base...); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func writeTestJSON(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
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
	adapter, client := newTestClient(t, server, false, true)
	if adapter.Name() != adapterName || client.Platform() != "dingtalk" || client.Account() != "main" {
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
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, CapabilityContacts, CapabilityRobotMessages} {
		if !capabilities.Has(capability) || capabilities[capability].DocURL == "" {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage,
		socialhub.CapWebhook, CapabilityAppToken,
	} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %s=%#v", capability, capabilities[capability])
		}
	}
	if fetcher, ok := client.Fetcher(); !ok || fetcher == nil {
		t.Fatal("fetcher should be available")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("common messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if client.ContactWorkflow() == nil || client.RobotWorkflow() == nil || client.AuthWorkflow() == nil {
		t.Fatal("typed workflows should be available")
	}
	if _, err := client.RefreshAppToken(context.Background()); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("static token refresh=%v", err)
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
	if err := adapter.Init(context.Background(), testConfig(server.URL, false, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationAndCredentialResolution(t *testing.T) {
	valid := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "token", Settings: map[string]any{"corp_id": "corp"},
	}
	invalid := []socialhub.AdapterConfig{
		{Adapter: adapterName},
		{Adapter: "dingtalk", Accounts: []socialhub.AccountConfig{valid}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{valid}},
		{Adapter: adapterName, Settings: map[string]any{"unknown": true}, Accounts: []socialhub.AccountConfig{valid}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token"}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", ClientID: "client", Settings: map[string]any{"corp_id": "corp"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"corp_id": "corp", "unknown": true}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"corp_id": "corp"}, Webhook: socialhub.WebhookConfig{SecretRef: "secret"}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, false, false), socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	config := testConfig(server.URL, false, false)
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithSecretResolver(testResolver{"test://access-token": strings.Repeat("x", 4097)})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oversized access token=%v", err)
	}
}
