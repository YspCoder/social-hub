package tumblr

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

var testNow = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

func testConfig(serverURL string, userToken bool, scopes []string) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", ClientID: "tumblr-key", SecretRef: "test://client-secret",
		Settings: map[string]any{"blog_identifier": "example.tumblr.com"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	if userToken {
		account.AccessTokenRef = "test://access-token"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Settings: map[string]any{
			"base_url": serverURL, "auth_url": serverURL + "/oauth2/authorize", "token_url": serverURL + "/oauth2/token",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server, userToken bool, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), testConfig(server.URL, userToken, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://client-secret": "client-secret", "test://access-token": "access-token",
		}),
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

func writeEnvelope(t *testing.T, writer http.ResponseWriter, response any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(map[string]any{
		"meta": map[string]any{"status": 200, "msg": "OK"}, "response": response,
	}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, true, []string{"basic", "write", "offline_access"})
	if adapter.Name() != adapterName || client.Platform() != "tumblr" || client.Account() != "main" {
		t.Fatalf("identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	metadata := adapter.Metadata()
	if metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, CapabilityNPF, CapabilityTimeline, CapabilityEngagement} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %s=%#v", capability, capabilities[capability])
		}
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if client.NPFWorkflow() == nil || client.TimelineWorkflow() == nil || client.EngagementWorkflow() == nil {
		t.Fatal("typed workflows unavailable")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	oauth, err := adapter.OAuth(context.Background(), "main")
	if err != nil || oauth.ClientID != "tumblr-key" || oauth.ClientSecret != "client-secret" {
		t.Fatalf("oauth=%#v error=%v", oauth, err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("oauth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, true, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationPublicClientAndScopeGating(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	invalid := []socialhub.AdapterConfig{
		{Adapter: "tumblr", Accounts: []socialhub.AccountConfig{{ID: "main", ClientID: "key", Settings: map[string]any{"blog_identifier": "blog"}}}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{{ID: "main", ClientID: "key", Settings: map[string]any{"blog_identifier": "blog"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", Settings: map[string]any{"blog_identifier": "blog"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", ClientID: "key", Settings: map[string]any{"blog_identifier": "bad/blog"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", ClientID: "key", Settings: map[string]any{"unknown": true}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}

	adapter, client := newTestAdapter(t, server, false, []string{"basic"})
	if _, ok := client.Publisher(); ok {
		t.Fatal("public client exposes publisher")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[socialhub.CapPublish].Supported || capabilities[CapabilityEngagement].Supported || capabilities[CapabilityTimeline].Approval != socialhub.ApprovalGranted {
		t.Fatalf("public capabilities=%#v", capabilities)
	}
	if _, err := client.requireUser("test"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("require user=%v", err)
	}
	if err := client.requireScopes("test", "write"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope gating=%v", err)
	}
	client.scopes = nil
	if err := client.requireScopes("test", "write"); err != nil {
		t.Fatalf("unknown scopes should defer to platform: %v", err)
	}
	if selected, err := client.selectedBlog(""); err != nil || selected != "example.tumblr.com" {
		t.Fatalf("default blog=%q error=%v", selected, err)
	}
	if _, err := client.selectedBlog("bad/blog"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid blog=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing oauth account=%v", err)
	}
}
