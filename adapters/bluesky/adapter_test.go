package bluesky

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

var testNow = time.Date(2026, time.August, 1, 12, 30, 0, 123000000, time.UTC)

func testConfig(serviceURL string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName,
		Accounts: []socialhub.AccountConfig{{
			ID: "main", SecretRef: "test://password", AccessTokenRef: "test://access",
			Settings: map[string]any{
				"service_url": serviceURL,
				"repo":        "did:plc:alice",
				"identifier":  "alice.test",
			},
		}},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://access":   "access-token",
			"test://password": "app-password",
		}),
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

func TestAdapterRegistrationCapabilitiesAndSession(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server)

	if client.Platform() != "bluesky" || client.Account() != "main" {
		t.Fatalf("identity=%s/%s", client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact,
		CapabilityHomeTimeline, CapabilityPostRecord,
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
	if client.TimelineWorkflow() == nil || client.PostRecordWorkflow() == nil {
		t.Fatal("typed workflows should be available")
	}
	metadata := adapter.Metadata()
	if metadata.Name != adapterName || metadata.APIVersion != apiVersion || metadata.Product != productName || metadata.DocURL != docURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	session, err := adapter.Session(context.Background(), "main")
	if err != nil || session.ServiceURL != server.URL || session.Identifier != "alice.test" || session.Password != "app-password" {
		t.Fatalf("session=%#v error=%v", session, err)
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
	if _, err := adapter.Session(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("session after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinitialize closed adapter error=%v", err)
	}
}

func TestAdapterValidationAndMissingCredentials(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	invalid := []socialhub.AdapterConfig{
		{Adapter: "bluesky", Accounts: []socialhub.AccountConfig{{ID: "one", Settings: map[string]any{"service_url": server.URL}}}},
		{Adapter: adapterName, Settings: map[string]any{"service_url": server.URL}, Accounts: []socialhub.AccountConfig{{ID: "one", Settings: map[string]any{"service_url": server.URL}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", Settings: map[string]any{"service_url": server.URL + "/path"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", Settings: map[string]any{"service_url": server.URL, "repo": "alice.test"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "one", Settings: map[string]any{"service_url": server.URL, "unknown": true}}}},
	}
	for _, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("config=%#v error=%v", config, err)
		}
	}

	config := testConfig(server.URL)
	config.Accounts[0].AccessTokenRef = ""
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.Session(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing session account error=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token reference error=%v", err)
	}

	config = testConfig(server.URL)
	config.Accounts[0].Settings["repo"] = ""
	config.Accounts[0].Settings["identifier"] = "alice.test"
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://access": "access-token", "test://password": "app-password"}),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing repo error=%v", err)
	}
	session, err := adapter.Session(context.Background(), "main")
	if err != nil || session.Identifier != "alice.test" {
		t.Fatalf("session-only config=%#v error=%v", session, err)
	}
}

func TestAdapterUsesPerAccountPDS(t *testing.T) {
	newPDS := func(did, handle string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/xrpc/app.bsky.actor.getProfile" || request.Header.Get("Authorization") != "Bearer "+handle+"-token" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"did": did, "handle": handle})
		}))
	}
	first := newPDS("did:plc:first", "first.test")
	defer first.Close()
	second := newPDS("did:plc:second", "second.test")
	defer second.Close()
	config := socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{
		{ID: "first", AccessTokenRef: "first-token", Settings: map[string]any{"service_url": first.URL, "repo": "did:plc:first"}},
		{ID: "second", AccessTokenRef: "second-token", Settings: map[string]any{"service_url": second.URL, "repo": "did:plc:second"}},
	}}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(&http.Client{}),
		socialhub.WithSecretResolver(testResolver{"first-token": "first.test-token", "second-token": "second.test-token"}),
	); err != nil {
		t.Fatal(err)
	}
	for _, accountID := range []socialhub.AccountID{"first", "second"} {
		common, err := adapter.Client(context.Background(), accountID)
		if err != nil {
			t.Fatal(err)
		}
		user, err := common.(*Client).GetUser(context.Background(), "")
		if err != nil || user.ID != "did:plc:"+string(accountID) {
			t.Fatalf("account=%s user=%#v error=%v", accountID, user, err)
		}
	}
}
