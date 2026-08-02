package discourse

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

func testConfig(server *httptest.Server, api, webhook bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "forum", Settings: map[string]any{"base_url": server.URL, "api_username": "system"},
	}
	if api {
		account.AccessTokenRef = "test://api-key"
	}
	if webhook {
		account.Webhook.SecretRef = "test://webhook-secret"
	}
	return socialhub.AdapterConfig{Adapter: adapterName, Product: productName, Accounts: []socialhub.AccountConfig{account}}
}

func newTestClient(t *testing.T, server *httptest.Server, api, webhook bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, api, webhook),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://api-key": "api-key", "test://webhook-secret": "webhook-secret",
		}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 2, 3, 4, 5, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "forum")
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
	adapter, client := newTestClient(t, server, true, true)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact,
		socialhub.CapWebhook, CapabilityTopics, CapabilityPrivateMessages,
	} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMessage) {
		t.Fatal("common messenger must not be advertised")
	}
	if client.Platform() != "discourse" || client.Account() != "forum" || client.TopicWorkflow() == nil || client.PrivateMessageWorkflow() == nil || client.Close() != nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("Publisher must be exposed")
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
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("WebhookHandler must be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger must not be exposed")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "forum"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestWebhookOnlyClientAndValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, true)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("Fetcher must not be exposed without an API key")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher must not be exposed without an API key")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader must not be exposed without an API key")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("Reactor must not be exposed without an API key")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("WebhookHandler must be exposed")
	}
	if _, err := client.GetUser(context.Background(), "system"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing API=%v", err)
	}

	valid := testConfig(server, true, true)
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
		{"username", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["api_username"] = "" }},
		{"credentials", func(config *socialhub.AdapterConfig) {
			config.Accounts[0].AccessTokenRef = ""
			config.Accounts[0].Webhook.SecretRef = ""
		}},
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

	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "forum"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing secret=%v", err)
	}
}

func TestAdapterSecurityAndValidationHelpers(t *testing.T) {
	if !validBaseURL("http://localhost/forum") || validBaseURL("ftp://example.test") || validBaseURL("https://example.test/#fragment") {
		t.Fatal("base URL validation failed")
	}
	if !validHeaderValue("system", 10) || validHeaderValue("bad\nvalue", 20) || validHeaderValue("", 2) {
		t.Fatal("header validation failed")
	}
	if !validUsername("user_name") || validUsername("bad/user") || validUsername("") || !validID("123") || validID("0") || validID("abc") {
		t.Fatal("identifier validation failed")
	}
	if !validCursor("") || !validCursor("12") || validCursor("bad") || path("posts", "12") != "/posts/12.json" || !validText("text", 4) || validText(" ", 4) {
		t.Fatal("request validation failed")
	}

	auth := apiKeyAuthenticator{username: "system"}
	request := httptest.NewRequest(http.MethodGet, "https://forum.example/posts.json", nil)
	if err := auth.Authenticate(request, socialhub.Token{AccessToken: "api-key"}); err != nil || request.Header.Get("Api-Key") != "api-key" || request.Header.Get("Api-Username") != "system" {
		t.Fatalf("authentication headers=%v err=%v", request.Header, err)
	}
	if err := (apiKeyAuthenticator{}).Authenticate(request, socialhub.Token{AccessToken: "api-key"}); err == nil {
		t.Fatal("empty API username must fail")
	}

	origin, _ := http.NewRequest(http.MethodGet, "https://forum.example/start", nil)
	same, _ := http.NewRequest(http.MethodGet, "https://forum.example/next", nil)
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
	if boundedMessage(strings.Repeat("界", 3), 2) != "界界" || firstNonEmpty("", " value ") != " value " {
		t.Fatal("message helpers failed")
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
