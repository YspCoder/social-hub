package vk

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

func testConfig(baseURL string, kind TokenKind, ownerID int64, webhook bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "test://access-token",
		Settings: map[string]any{"owner_id": ownerID, "token_kind": string(kind)},
	}
	if webhook {
		account.Webhook.SecretRef = "test://callback-secret"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Settings: map[string]any{"base_url": baseURL + "/method"},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server, kind TokenKind, ownerID int64, webhook bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), testConfig(server.URL, kind, ownerID, webhook),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://access-token":    "access-token",
			"test://callback-secret": "callback-secret",
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

func writeTestJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func requireErrorCode(t *testing.T, err error, code socialhub.ErrorCode) *socialhub.Error {
	t.Helper()
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != code {
		t.Fatalf("error=%#v, want code %s", err, code)
	}
	return platformErr
}

func TestAdapterRegistrationMetadataAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, TokenUser, 123, false)
	if adapter.Name() != adapterName || client.Platform() != "vk" || client.Account() != "main" {
		t.Fatalf("identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	metadata := adapter.Metadata()
	if metadata.Name != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != docURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	if client.WallWorkflow() == nil || client.CallbackWorkflow() == nil {
		t.Fatal("typed workflows must be available")
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
	if err := adapter.Init(context.Background(), testConfig(server.URL, TokenUser, 123, false)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
	if _, err := (&Adapter{}).Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client before init=%v", err)
	}
}

func TestTokenKindCapabilityMatrix(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	tests := []struct {
		name     string
		kind     TokenKind
		ownerID  int64
		webhook  bool
		publish  bool
		media    bool
		react    bool
		message  bool
		callback bool
	}{
		{name: "user", kind: TokenUser, ownerID: 123, publish: true, media: true, react: true, message: true},
		{name: "community", kind: TokenCommunity, ownerID: -456, webhook: true, react: true, message: true, callback: true},
		{name: "service", kind: TokenService, ownerID: 789},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, client := newTestAdapter(t, server, test.kind, test.ownerID, test.webhook)
			capabilities, err := client.Capabilities(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := map[socialhub.Capability]bool{
				socialhub.CapPublish: test.publish, socialhub.CapFetch: true, socialhub.CapMedia: test.media,
				socialhub.CapReact: test.react, socialhub.CapMessage: test.message, socialhub.CapWebhook: test.callback,
				CapabilityWall: test.publish, CapabilityCallback: test.ownerID < 0,
			}
			for capability, supported := range want {
				state, ok := capabilities[capability]
				if !ok || state.Capability != capability || state.Supported != supported || state.Reason == "" {
					t.Fatalf("capability %s=%#v", capability, state)
				}
			}
			if _, ok := client.Publisher(); ok != test.publish {
				t.Fatalf("publisher=%v", ok)
			}
			if _, ok := client.Fetcher(); !ok {
				t.Fatal("fetcher must be available")
			}
			if _, ok := client.MediaUploader(); ok != test.media {
				t.Fatalf("media=%v", ok)
			}
			if _, ok := client.Reactor(); ok != test.react {
				t.Fatalf("reactor=%v", ok)
			}
			if _, ok := client.Messenger(); ok != test.message {
				t.Fatalf("messenger=%v", ok)
			}
			if _, ok := client.WebhookHandler(); ok != test.callback {
				t.Fatalf("webhook=%v", ok)
			}
		})
	}
}

func TestAdapterValidationAndSecretResolution(t *testing.T) {
	validAccount := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "token",
		Settings: map[string]any{"owner_id": int64(123), "token_kind": string(TokenUser)},
	}
	invalid := []socialhub.AdapterConfig{
		{},
		{Adapter: "vk", Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Settings: map[string]any{"unknown": true}, Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", Settings: validAccount.Settings}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"owner_id": 0, "token_kind": string(TokenUser)}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"owner_id": 1, "token_kind": "bot"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"owner_id": 1, "token_kind": string(TokenCommunity)}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Webhook: socialhub.WebhookConfig{SecretRef: "secret"}, Settings: map[string]any{"owner_id": 1, "token_kind": string(TokenUser)}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"owner_id": 1, "token_kind": string(TokenUser), "unknown": true}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, TokenUser, 123, false), socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token=%v", err)
	}
	if validEndpoint("://bad") || validEndpoint("ftp://example.test") || validEndpoint("https://user@example.test") || !validEndpoint("https://example.test/method") {
		t.Fatal("endpoint validation mismatch")
	}
}
