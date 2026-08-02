package lemmy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(server *httptest.Server) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Accounts: []socialhub.AccountConfig{{
			ID: "alice", AccessTokenRef: "test://jwt",
			Settings: map[string]any{"base_url": server.URL, "username": "alice"},
		}},
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = cloneMap(input.Settings)
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	for index := range output.Accounts {
		output.Accounts[index].Settings = cloneMap(input.Accounts[index].Settings)
	}
	return output
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(
		context.Background(), testConfig(server),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://jwt": " jwt-token "}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 2, 4, 5, 6, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Name != adapterName || metadata.Product != productName ||
		metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact,
		CapabilityPosts, CapabilityVotes, CapabilityPrivateMessages,
	} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unexpected capability %s", capability)
		}
	}
	if client.Platform() != "lemmy" || client.Account() != "alice" || client.PostWorkflow() == nil ||
		client.VoteWorkflow() == nil || client.PrivateMessageWorkflow() == nil || client.Close() != nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.MediaUploader(); !ok {
		t.Fatal("MediaUploader must be exposed")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("Reactor must be exposed")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("WebhookHandler must not be exposed")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "alice"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestAdapterValidationAndSecrets(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server)
	invalid := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"empty accounts", func(config *socialhub.AdapterConfig) { config.Accounts = nil }},
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"global setting", func(config *socialhub.AdapterConfig) { config.Settings = map[string]any{"unknown": true} }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
		{"base scheme", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["base_url"] = "ftp://example.test" }},
		{"base credentials", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["base_url"] = "https://u:p@example.test"
		}},
		{"base path", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["base_url"] = "https://example.test/lemmy"
		}},
		{"base query", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["base_url"] = "https://example.test?x=1"
		}},
		{"username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["username"] = "bad/name" }},
		{"token reference", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(valid)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	uninitialized := &Adapter{}
	if _, err := uninitialized.Client(context.Background(), "alice"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("uninitialized client=%v", err)
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "alice"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing JWT=%v", err)
	}

	badJWT := &Adapter{}
	if err := badJWT.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://jwt": "bad token"})); err != nil {
		t.Fatal(err)
	}
	if _, err := badJWT.Client(context.Background(), "alice"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("invalid JWT=%v", err)
	}
}

func TestSecurityPaginationAndValidationHelpers(t *testing.T) {
	if !validBaseURL("http://localhost/") || validBaseURL("https://example.test/path") || validBaseURL("https://example.test/#x") {
		t.Fatal("base URL validation failed")
	}
	if !validHeaderValue("jwt-token", 20) || validHeaderValue("bad token", 20) || validHeaderValue("", 20) {
		t.Fatal("header validation failed")
	}
	if !validUsername("alice@example.test") || validUsername("bad user") || validUsername("") {
		t.Fatal("username validation failed")
	}
	if !validCursor("opaque cursor") || validCursor("bad\ncursor") || !validID("9") || validID("0") || validID("a") {
		t.Fatal("cursor or ID validation failed")
	}
	if !validTitle("A title") || validTitle("no") || validTitle("bad\ntitle") || !validBody("hello", 5) || validBody("hello!", 5) {
		t.Fatal("content validation failed")
	}
	if !validPostURL("https://example.test/post") || !validPostURL("magnet:?xt=urn:test") || validPostURL("ftp://example.test/x") ||
		!validHTTPURL("https://example.test/x") || validHTTPURL("https://u:p@example.test/x") {
		t.Fatal("URL validation failed")
	}
	query, page, size, err := pageQuery("2", 10)
	if err != nil || query.Get("page") != "2" || query.Get("limit") != "10" || page != 2 || size != 10 {
		t.Fatalf("page query=%v page=%d size=%d err=%v", query, page, size, err)
	}
	if _, _, _, err := pageQuery("bad", 1); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad cursor=%v", err)
	}
	if _, _, _, err := pageQuery("", 51); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad size=%v", err)
	}
	next, previous, more := pageCursors(10, 2, 10)
	if next == nil || *next != "3" || previous == nil || *previous != "1" || !more {
		t.Fatalf("cursors next=%v previous=%v more=%v", next, previous, more)
	}
	if next, previous, more := pageCursors(1, 1, 10); next != nil || previous != nil || more {
		t.Fatalf("last page cursors next=%v previous=%v more=%v", next, previous, more)
	}

	origin, _ := http.NewRequest(http.MethodGet, "https://lemmy.example/start", nil)
	same, _ := http.NewRequest(http.MethodGet, "https://lemmy.example/next", nil)
	cross, _ := http.NewRequest(http.MethodGet, "https://other.example/next", nil)
	if err := rejectCrossOriginRedirect(same, []*http.Request{origin}); err != nil {
		t.Fatalf("same-origin redirect=%v", err)
	}
	if err := rejectCrossOriginRedirect(cross, []*http.Request{origin}); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("cross-origin redirect=%v", err)
	}
	via := make([]*http.Request, 10)
	for index := range via {
		via[index] = origin
	}
	if err := rejectCrossOriginRedirect(same, via); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("redirect limit=%v", err)
	}
}
