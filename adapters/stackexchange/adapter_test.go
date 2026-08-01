package stackexchange

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type mapResolver map[string]string

func (resolver mapResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, exists := resolver[reference]
	if !exists {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type mutableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *mutableClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *mutableClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

func testConfig(server *httptest.Server, withToken bool, scopes []string) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "stackoverflow", AppID: "app-key", ClientID: "12345", SecretRef: "test://client-secret",
		Settings: map[string]any{"site": "stackoverflow", "user_id": "42"}, Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	if withToken {
		account.AccessTokenRef = "test://access-token"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{"base_url": server.URL + "/2.3", "auth_url": server.URL + "/oauth", "token_url": server.URL + "/oauth/access_token/json"},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, withToken bool, scopes []string, clock *mutableClock) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server, withToken, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(mapResolver{"test://client-secret": "client-secret", "test://access-token": "access-token"}),
		socialhub.WithClock(clock),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "stackoverflow")
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
	clock := &mutableClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	adapter, client := newTestClient(t, server, true, []string{"write_access", "no_expiry"}, clock)
	metadata := adapter.Metadata()
	if adapter.Name() != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != documentationURL {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(CapabilityQnA) || !capabilities.Has(socialhub.CapFetch) || !capabilities.Has(socialhub.CapReact) {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if capabilities.Has(socialhub.CapPublish) || capabilities.Has(socialhub.CapMedia) || capabilities.Has(socialhub.CapMessage) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if client.Platform() != "stackexchange" || client.Account() != "stackoverflow" || client.QnAWorkflow() == nil || client.Close() != nil {
		t.Fatalf("client=%#v", client)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("common Publisher must not be exposed")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("Fetcher must be exposed")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("authenticated Reactor must be exposed")
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
	if _, err := adapter.Client(context.Background(), "stackoverflow"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close error=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server, true, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("reinit error=%v", err)
	}
}

func TestPublicClientAndAdapterValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	clock := &mutableClock{now: time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}
	_, client := newTestClient(t, server, false, nil, clock)
	capabilities, _ := client.Capabilities(context.Background())
	if !capabilities.Has(socialhub.CapFetch) || capabilities.Has(socialhub.CapReact) || capabilities.Has(CapabilityQnA) {
		t.Fatalf("public capabilities=%#v", capabilities)
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("public client must not expose Reactor")
	}

	valid := testConfig(server, false, nil)
	tests := []struct {
		name   string
		mutate func(*socialhub.AdapterConfig)
	}{
		{"adapter mismatch", func(config *socialhub.AdapterConfig) { config.Adapter = "other" }},
		{"missing key", func(config *socialhub.AdapterConfig) { config.Accounts[0].AppID = "" }},
		{"bad site", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["site"] = "stack overflow" }},
		{"bad user", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["user_id"] = "../42" }},
		{"bad endpoint", func(config *socialhub.AdapterConfig) {
			config.Settings["base_url"] = "https://user:pass@example.com/2.3"
		}},
		{"unknown setting", func(config *socialhub.AdapterConfig) { config.Accounts[0].Settings["unknown"] = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			config.Settings = cloneMap(valid.Settings)
			config.Accounts = append([]socialhub.AccountConfig(nil), valid.Accounts...)
			config.Accounts[0].Settings = cloneMap(valid.Accounts[0].Settings)
			test.mutate(&config)
			if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(mapResolver{}), socialhub.WithClock(clock)); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account error=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing OAuth account error=%v", err)
	}
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
