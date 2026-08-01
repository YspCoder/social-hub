package wordpresscom

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

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

func testConfig(server *httptest.Server, authenticated bool, scopes []string) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "blog", ClientID: "client-id", SecretRef: "test://client-secret",
		Settings: map[string]any{"site": "123", "user_id": "7"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	if authenticated {
		account.AccessTokenRef = "test://access-token"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/rest/v1.1", "auth_url": server.URL + "/oauth2/authorize", "token_url": server.URL + "/oauth2/token",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, authenticated bool, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, authenticated, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret", "test://access-token": "access-token"}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "blog")
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
	adapter, client := newTestClient(t, server, true, []string{"global"})
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact, CapabilityPosts, CapabilitySite, CapabilityMedia} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMessage) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("unexpected capabilities=%#v", capabilities)
	}
	if client.Platform() != "wordpress.com" || client.Account() != "blog" || client.PostWorkflow() == nil || client.SiteWorkflow() == nil || client.MediaLibraryWorkflow() == nil || client.Close() != nil {
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
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger must not be exposed")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("WebhookHandler must not be exposed")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "blog"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "blog"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("oauth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestPublicClientAndValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, public := newTestClient(t, server, false, nil)
	capabilities, err := public.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapFetch) || capabilities.Has(socialhub.CapPublish) || capabilities.Has(socialhub.CapMedia) || capabilities.Has(socialhub.CapReact) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := public.Publisher(); ok {
		t.Fatal("public Publisher must not be exposed")
	}
	if _, ok := public.MediaUploader(); ok {
		t.Fatal("public MediaUploader must not be exposed")
	}
	if _, ok := public.Reactor(); ok {
		t.Fatal("public Reactor must not be exposed")
	}
	if _, err := public.GetUser(context.Background(), "me"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("public user error=%v", err)
	}

	valid := testConfig(server, true, nil)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"endpoint", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.test" }},
		{"site", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["site"] = "bad/path" }},
		{"user", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_id"] = "zero" }},
		{"unknown", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
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
		t.Fatalf("missing client=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing oauth=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "blog"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "blog"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing secret=%v", err)
	}

	invalidOAuth := cloneConfig(valid)
	invalidOAuth.Accounts[0].ClientID = ""
	invalidOAuth.Accounts[0].SecretRef = ""
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), invalidOAuth, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.OAuth(context.Background(), "blog"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete oauth=%v", err)
	}
}

func TestValidationHelpersAndScopeGating(t *testing.T) {
	if !validSite("example.wordpress.com") || !validSite("123") || validSite("localhost") || validSite("https://example.com") || validID("0") || validID("-1") || !validID("42") {
		t.Fatal("site or ID validation contract failed")
	}
	if validOpaque("", 2) || validOpaque("a b", 3) || !validOpaque("ab", 2) || validCursor("") || validCursor("bad\n") || !validCursor("cursor") {
		t.Fatal("opaque validation contract failed")
	}
	if !scopeGranted([]string{"auth"}, "users") || !scopeGranted([]string{"global"}, "media") || scopeGranted([]string{"read"}, "posts") {
		t.Fatal("scope contract failed")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, []string{"read"})
	if err := client.requireScopes("test", "posts"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
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
