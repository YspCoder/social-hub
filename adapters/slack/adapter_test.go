package slack

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

const (
	testWorkspaceID = "T123ABC"
	testChannelID   = "C123ABC"
	testPrivateID   = "G123ABC"
	testDMID        = "D123ABC"
	testActorID     = "U123ABC"
	testFileID      = "F123ABC"
	testTimestamp   = "1785571200.000001"
)

var testNow = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

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

func testConfig(baseURL, defaultChannel string, webhook bool, scopes []string) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "test://access-token", Approval: socialhub.ApprovalConfig{Scopes: scopes},
		Settings: map[string]any{
			"workspace_id": testWorkspaceID, "token_kind": string(TokenBot),
			"actor_id": testActorID, "default_channel_id": defaultChannel,
		},
	}
	if webhook {
		account.Webhook.SecretRef = "test://signing-secret"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Settings: map[string]any{"base_url": baseURL + "/api"},
		Accounts: []socialhub.AccountConfig{account},
	}
}

func allTestScopes() []string {
	return []string{
		"chat:write", "users:read", "files:write", "files:read", "reactions:write",
		"channels:history", "groups:history", "im:history", "mpim:history",
	}
}

func newTestAdapter(t *testing.T, server *httptest.Server, defaultChannel string, webhook bool, scopes []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, defaultChannel, webhook, scopes),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://access-token": "xoxb-test-token", "test://signing-secret": "signing-secret",
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

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestAdapter(t, server, testChannelID, true, allTestScopes())
	if adapter.Name() != adapterName || client.Platform() != "slack" || client.Account() != "main" {
		t.Fatalf("identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	metadata := adapter.Metadata()
	if metadata.Name != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != docURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact,
		socialhub.CapMessage, socialhub.CapWebhook, CapabilityChat, CapabilityFiles, CapabilityEvents,
	} {
		state := capabilities[name]
		if !state.Supported || state.Capability != name || state.Reason == "" {
			t.Fatalf("capability %s=%#v", name, state)
		}
		if len(state.Scopes) > 0 && state.Approval != socialhub.ApprovalGranted {
			t.Fatalf("capability approval %s=%#v", name, state)
		}
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
	}
	if _, ok := client.MediaUploader(); !ok {
		t.Fatal("media uploader unavailable")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger unavailable")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook unavailable")
	}
	if client.ChatWorkflow() == nil || client.FileWorkflow() == nil {
		t.Fatal("typed workflows unavailable")
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
	if err := adapter.Init(context.Background(), testConfig(server.URL, testChannelID, false, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestOptionalCapabilitiesAndScopeApproval(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "", false, []string{"users:read"})
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities[socialhub.CapPublish].Supported || capabilities[socialhub.CapWebhook].Supported || capabilities[CapabilityEvents].Supported {
		t.Fatalf("optional capabilities=%#v", capabilities)
	}
	if capabilities[socialhub.CapMedia].Approval != socialhub.ApprovalRequired || capabilities[socialhub.CapFetch].Approval != socialhub.ApprovalRequired {
		t.Fatalf("scope approval=%#v", capabilities)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher exposed without default channel")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook exposed without signing secret")
	}
	if err := client.requireAnyScope("history", "channels:history", "groups:history"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("any-scope validation=%v", err)
	}
	if err := client.requireScopes("files", "files:write"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope validation=%v", err)
	}
	_, unknown := newTestAdapter(t, server, testChannelID, false, nil)
	if err := unknown.requireScopes("chat", "chat:write"); err != nil {
		t.Fatalf("unknown scopes should defer to Slack: %v", err)
	}
}

func TestAdapterValidationAndSecretResolution(t *testing.T) {
	valid := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "token",
		Settings: map[string]any{"workspace_id": testWorkspaceID, "token_kind": string(TokenBot)},
	}
	invalid := []socialhub.AdapterConfig{
		{},
		{Adapter: "slack", Accounts: []socialhub.AccountConfig{valid}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: []socialhub.AccountConfig{valid}},
		{Adapter: adapterName, Settings: map[string]any{"unknown": true}, Accounts: []socialhub.AccountConfig{valid}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", Settings: valid.Settings}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"workspace_id": "bad", "token_kind": string(TokenBot)}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"workspace_id": testWorkspaceID, "token_kind": "app"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"workspace_id": testWorkspaceID, "token_kind": string(TokenUser), "actor_id": "bad"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"workspace_id": testWorkspaceID, "token_kind": string(TokenUser), "default_channel_id": "U123"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"workspace_id": testWorkspaceID, "token_kind": string(TokenUser), "unknown": true}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, testChannelID, false, nil), socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token=%v", err)
	}
	if validEndpoint("://bad") || validEndpoint("ftp://example.test") || validEndpoint("https://user@example.test") || !validEndpoint("https://slack.com/api") {
		t.Fatal("endpoint validation mismatch")
	}
	if validSlackID("bad", "T") || validSlackID("Tbad", "T") || !validSlackID(testWorkspaceID, "T") {
		t.Fatal("Slack ID validation mismatch")
	}
}
