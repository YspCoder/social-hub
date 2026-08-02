package patreon

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

func testConfig(server *httptest.Server, token, webhook bool, scopes []string) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "creator", ClientID: "client-id", SecretRef: "test://client-secret",
		Settings: map[string]any{"campaign_id": "100", "user_id": "10"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	if token {
		account.AccessTokenRef = "test://access-token"
	}
	if webhook {
		account.Webhook.SecretRef = "test://webhook-secret"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": server.URL + "/api/oauth2/v2", "auth_url": server.URL + "/oauth2/authorize", "token_url": server.URL + "/api/oauth2/token",
		},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, token, webhook bool, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, token, webhook, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{
			"test://client-secret": "client-secret", "test://access-token": "access-token", "test://webhook-secret": "webhook-secret",
		}),
		socialhub.WithClock(fixedClock{now: time.Date(2026, 8, 2, 2, 3, 4, 0, time.UTC)}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "creator")
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
	scopes := []string{"identity", "campaigns", "campaigns.posts", "campaigns.members"}
	adapter, client := newTestClient(t, server, true, true, scopes)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, socialhub.CapWebhook, CapabilityCampaigns, CapabilityMembers, CapabilityPosts} {
		if !capabilities.Has(capability) || capabilities[capability].Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact, socialhub.CapMessage} {
		if capabilities.Has(capability) {
			t.Fatalf("unexpected capability %s", capability)
		}
	}
	if client.Platform() != "patreon" || client.Account() != "creator" || client.CampaignWorkflow() == nil || client.MemberWorkflow() == nil || client.Close() != nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("WebhookHandler must be exposed")
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("Publisher must not be exposed")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("MediaUploader must not be exposed")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("Reactor must not be exposed")
	}
	if _, ok := client.Messenger(); ok {
		t.Fatal("Messenger must not be exposed")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "creator"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "creator"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("oauth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, true, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit=%v", err)
	}
}

func TestWebhookOnlyClientAndValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, true, nil)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Fetcher(); ok {
		t.Fatal("Fetcher must not be exposed without a token")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("WebhookHandler must be exposed")
	}
	if _, err := client.GetUser(context.Background(), "me"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing API=%v", err)
	}

	valid := testConfig(server, true, true, nil)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"endpoint credentials", func(config *socialhub.AdapterConfig) { config.Settings["base_url"] = "https://user:pass@example.test" }},
		{"endpoint query", func(config *socialhub.AdapterConfig) { config.Settings["token_url"] = "https://example.test/token?x=1" }},
		{"campaign", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["campaign_id"] = "bad/path" }},
		{"user", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_id"] = "bad user" }},
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
	if _, err := adapter.Client(context.Background(), "creator"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "creator"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing client secret=%v", err)
	}

	incomplete := cloneConfig(valid)
	incomplete.Accounts[0].ClientID = ""
	incomplete.Accounts[0].SecretRef = ""
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), incomplete, socialhub.WithHTTPClient(server.Client())); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.OAuth(context.Background(), "creator"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("incomplete oauth=%v", err)
	}
}

func TestValidationAndScopeHelpers(t *testing.T) {
	if !validResourceID("member-123") || validResourceID("") || validResourceID("bad/path") || validResourceID("bad user") {
		t.Fatal("resource ID validation failed")
	}
	if !validOpaque("state", 5) || validOpaque("a b", 3) || validOpaque("", 2) || !validCursor("opaque:cursor") || validCursor("bad\n") {
		t.Fatal("opaque validation failed")
	}
	if !scopeGranted([]string{" identity "}, "identity") || scopeGranted([]string{"campaigns"}, "identity") || webhookApproval("") != socialhub.ApprovalUnknown || webhookApproval("secret") != socialhub.ApprovalGranted {
		t.Fatal("scope validation failed")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true, false, []string{"identity"})
	if err := client.requireScopes("test", "campaigns.posts"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope error=%v", err)
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook must not be exposed without a secret")
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
