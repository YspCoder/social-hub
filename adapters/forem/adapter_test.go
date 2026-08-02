package forem

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
			ID: "author", AccessTokenRef: "test://api-key", Settings: map[string]any{"base_url": server.URL},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://api-key": " api-key "}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 2, 4, 5, 6, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "author")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, socialhub.CapReact, CapabilityArticles, CapabilityReactions} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapMessage, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unexpected capability %s", capability)
		}
	}
	if client.Platform() != "forem" || client.Account() != "author" || client.ArticleWorkflow() == nil || client.ReactionWorkflow() == nil || client.Close() != nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("Reactor must be exposed")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader must not be exposed")
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
	if _, err := adapter.Client(context.Background(), "author"); !errors.Is(err, socialhub.ErrConflict) {
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
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"global settings", func(config *socialhub.AdapterConfig) { config.Settings = map[string]any{"unknown": true} }},
		{"base credentials", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["base_url"] = "https://user:pass@example.test"
		}},
		{"base query", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].Settings["base_url"] = "https://example.test?x=1"
		}},
		{"token reference", func(config *socialhub.AdapterConfig) { config.Accounts[0].AccessTokenRef = "" }},
		{"unknown account setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := cloneConfig(valid)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	defaultConfig := cloneConfig(valid)
	defaultConfig.Accounts[0].Settings = nil
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), defaultConfig); err != nil {
		t.Fatalf("default DEV config=%v", err)
	}

	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "author"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing API key=%v", err)
	}

	badKey := &Adapter{}
	if err := badKey.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{"test://api-key": "bad\nkey"})); err != nil {
		t.Fatal(err)
	}
	if _, err := badKey.Client(context.Background(), "author"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("invalid API key=%v", err)
	}
}

func TestSecurityAndRequestHelpers(t *testing.T) {
	if !validBaseURL("http://localhost/community") || validBaseURL("ftp://example.test") || validBaseURL("https://example.test/#fragment") {
		t.Fatal("base URL validation failed")
	}
	if !validHeaderValue("api-key", 10) || validHeaderValue("bad\nkey", 20) || validHeaderValue("", 2) {
		t.Fatal("header validation failed")
	}
	auth := apiKeyAuthenticator{}
	request := httptest.NewRequest(http.MethodGet, "https://dev.to/api/users/me", nil)
	if err := auth.Authenticate(request, socialhub.Token{AccessToken: "api-key"}); err != nil || request.Header.Get("api-key") != "api-key" {
		t.Fatalf("auth headers=%v err=%v", request.Header, err)
	}
	if err := auth.Authenticate(request, socialhub.Token{}); err == nil {
		t.Fatal("empty API key must fail")
	}

	origin, _ := http.NewRequest(http.MethodGet, "https://dev.to/start", nil)
	same, _ := http.NewRequest(http.MethodGet, "https://dev.to/next", nil)
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
	if err := rejectCrossOriginRedirect(same, via); err == nil {
		t.Fatal("redirect limit must fail")
	}
	if err := rejectCrossOriginRedirect(same, nil); err != nil {
		t.Fatalf("initial request=%v", err)
	}

	query, page, size, err := pageQuery("2", 1000)
	if err != nil || query.Get("page") != "2" || query.Get("per_page") != "1000" || page != 2 || size != 1000 {
		t.Fatalf("page query=%v page=%d size=%d err=%v", query, page, size, err)
	}
	if _, _, _, err := pageQuery("bad", 1); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cursor=%v", err)
	}
	if _, _, _, err := pageQuery("", 1001); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("page size=%v", err)
	}
	next, previous, more := pageCursors(2, 2, 2)
	if next == nil || *next != "3" || previous == nil || *previous != "1" || !more {
		t.Fatalf("cursors next=%v previous=%v more=%v", next, previous, more)
	}
	if !validID("10") || validID("0") || validID("bad") || !validIdentifier("alice") || validIdentifier("bad/user") || !validCommentID("abc123") || validCommentID("bad id") {
		t.Fatal("identifier validation failed")
	}
	if resourcePath("articles", "10") != "/api/articles/10" || !validText("text", 4) || validText(" ", 4) {
		t.Fatal("request helpers failed")
	}
	if boundedMessage(strings.Repeat("\u754c", 3), 2) != "\u754c\u754c" || firstNonEmpty("", "x") != "x" || firstPositive(0, 2) != 2 {
		t.Fatal("error helpers failed")
	}
}

func cloneConfig(input socialhub.AdapterConfig) socialhub.AdapterConfig {
	output := input
	output.Settings = cloneMap(input.Settings)
	output.Accounts = append([]socialhub.AccountConfig(nil), input.Accounts...)
	output.Accounts[0].Settings = cloneMap(input.Accounts[0].Settings)
	return output
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}
