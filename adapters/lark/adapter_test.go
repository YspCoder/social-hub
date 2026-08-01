package lark

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	testAppID       = "cli_testapp"
	testTenantKey   = "tenant-test"
	testActorID     = "cli_testapp"
	testChatID      = "oc_testchat"
	testMessageID   = "om_testmessage"
	testReplyID     = "om_testreply"
	testThreadID    = "omt_testthread"
	testUserID      = "ou_testuser"
	testReactionID  = "reaction_test"
	testResourceKey = "file_testresource"
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

func allTestScopes() []string {
	return []string{
		"im:message", "im:message:send_as_bot", "im:message.send_as_user", "im:message:readonly",
		"im:message.group_msg", "im:resource", "im:message.reactions:write_only", "im:message.reactions:read",
		"contact:contact.base:readonly", "contact:user.base:readonly",
	}
}

func testConfig(baseURL string, tokenKind TokenKind, defaultChat, actorID string, encrypted bool) socialhub.AdapterConfig {
	account := socialhub.AccountConfig{
		ID: "main", AppID: testAppID, AccessTokenRef: "test://access-token",
		Approval: socialhub.ApprovalConfig{Scopes: allTestScopes()},
		Webhook:  socialhub.WebhookConfig{TokenRef: "test://verification-token"},
		Settings: map[string]any{
			"region": string(RegionFeishu), "token_kind": string(tokenKind), "user_id_type": string(UserIDOpenID),
			"actor_id": actorID, "tenant_key": testTenantKey, "default_chat_id": defaultChat,
		},
	}
	if encrypted {
		account.Webhook.AESKeyRef = "test://encrypt-key"
	}
	settings := map[string]any{}
	if baseURL != "" {
		settings["base_url"] = baseURL
	}
	return socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{account}, Settings: settings}
}

func newTestClient(t *testing.T, server *httptest.Server, tokenKind TokenKind, defaultChat, actorID string, encrypted bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	resolver := testResolver{
		"test://access-token":       string(tokenKind) + "-access-token",
		"test://verification-token": "verification-token",
		"test://encrypt-key":        "encrypt-key-for-contract-tests",
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, tokenKind, defaultChat, actorID, encrypted),
		socialhub.WithHTTPClient(server.Client()), socialhub.WithSecretResolver(resolver), socialhub.WithClock(fixedClock{now: testNow})); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	return adapter, common.(*Client)
}

func requireLarkRequest(t *testing.T, writer http.ResponseWriter, request *http.Request, tokenKind TokenKind) map[string]any {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer "+string(tokenKind)+"-access-token" || request.Header.Get("Accept") != "application/json" {
		t.Errorf("unexpected request headers: %v", request.Header)
		http.Error(writer, "bad headers", http.StatusBadRequest)
		return nil
	}
	if request.Method == http.MethodPost || request.Method == http.MethodPut {
		if request.Header.Get("Content-Type") != "application/json; charset=utf-8" || request.Header.Get("Idempotency-Key") != "" {
			t.Errorf("unexpected content headers: %v", request.Header)
			http.Error(writer, "bad content headers", http.StatusBadRequest)
			return nil
		}
	}
	if request.Body == nil {
		return map[string]any{}
	}
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}
		}
		t.Errorf("decode request: %v", err)
		http.Error(writer, "bad JSON", http.StatusBadRequest)
		return nil
	}
	return body
}

func writeTestJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Tt-Logid", "log-id-test")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func requireErrorCode(t *testing.T, err error, code socialhub.ErrorCode) *socialhub.Error {
	t.Helper()
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != code {
		t.Fatalf("error=%v, want code %s", err, code)
	}
	return platformErr
}

func TestAdapterRegistrationMetadataAndLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	}))
	defer server.Close()

	adapter, err := socialhub.Open(context.Background(), adapterName, testConfig(server.URL, TokenTenant, testChatID, testActorID, false),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{
			"test://access-token": "tenant-access-token", "test://verification-token": "verification-token",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	metadata := adapter.Metadata()
	if metadata.Name != adapterName || metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != docURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	client, err := adapter.Client(context.Background(), "main")
	if err != nil || client.Platform() != "lark" || client.Account() != "main" {
		t.Fatalf("client=%#v err=%v", client, err)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities[socialhub.CapPublish].Supported || !capabilities[socialhub.CapMedia].Supported || !capabilities[socialhub.CapReact].Supported || !capabilities[socialhub.CapWebhook].Supported {
		t.Fatalf("capabilities=%#v err=%v", capabilities, err)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher missing")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher missing")
	}
	if _, ok := client.MediaUploader(); !ok {
		t.Fatal("media uploader missing")
	}
	if _, ok := client.Reactor(); !ok {
		t.Fatal("reactor missing")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger missing")
	}
	if _, ok := client.WebhookHandler(); !ok {
		t.Fatal("webhook handler missing")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); err == nil {
		t.Fatal("closed adapter created a client")
	}
}

func TestAdapterRegionAndUserTokenCapabilities(t *testing.T) {
	if baseURLFor(RegionFeishu) != defaultFeishuBaseURL || baseURLFor(RegionLark) != defaultLarkBaseURL {
		t.Fatal("region base URL mapping mismatch")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, TokenUser, "", "", false)
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capabilities[socialhub.CapPublish].Supported || capabilities[socialhub.CapMedia].Supported || capabilities[socialhub.CapReact].Supported {
		t.Fatalf("user capabilities=%#v", capabilities)
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher exposed without default chat")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media exposed for user token")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor exposed without actor ID")
	}
	if client.MessageWorkflow() == nil || client.ReactionWorkflow() == nil {
		t.Fatal("typed workflows missing")
	}
	if _, err := client.BeginUpload(context.Background(), socialhub.BeginUploadRequest{Filename: "x", Size: 1}); err == nil {
		t.Fatal("user token accepted media upload")
	}
}

func TestUserTokenCapabilityRequiresBothWriteScopes(t *testing.T) {
	partial := messageCapabilityState(TokenUser, socialhub.CapMessage, true, []string{"im:message"}, "", "")
	if partial.Approval != socialhub.ApprovalRequired {
		t.Fatalf("partial user scopes approval=%s", partial.Approval)
	}
	granted := messageCapabilityState(TokenUser, socialhub.CapMessage, true, []string{"im:message", "im:message.send_as_user"}, "", "")
	if granted.Approval != socialhub.ApprovalGranted {
		t.Fatalf("complete user scopes approval=%s", granted.Approval)
	}
	tenant := messageCapabilityState(TokenTenant, socialhub.CapMessage, true, []string{"im:message:send_as_bot"}, "", "")
	if tenant.Approval != socialhub.ApprovalGranted {
		t.Fatalf("tenant alternative scope approval=%s", tenant.Approval)
	}
}

func TestAdapterValidationAndClientFailures(t *testing.T) {
	valid := testConfig("https://open.feishu.cn", TokenTenant, testChatID, testActorID, false)
	tests := []socialhub.AdapterConfig{
		{},
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Adapter = "wrong" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Settings = map[string]any{"base_url": "://bad"} }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Settings = map[string]any{"unknown": true} }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Accounts[0].AccessTokenRef = "" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Accounts[0].Settings["region"] = "other" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Accounts[0].Settings["token_kind"] = "app" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Accounts[0].Settings["user_id_type"] = "bad" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Accounts[0].Settings["actor_id"] = "bad/id" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Accounts[0].Settings["tenant_key"] = "bad key" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) { value.Accounts[0].Settings["default_chat_id"] = "chat" }),
		mutatedConfig(func(value *socialhub.AdapterConfig) {
			value.Accounts[0].Webhook = socialhub.WebhookConfig{AESKeyRef: "key"}
		}),
	}
	for index, config := range tests {
		if err := (&Adapter{}).Init(context.Background(), config); err == nil {
			t.Fatalf("invalid config %d accepted", index)
		}
	}

	adapter := &Adapter{}
	if _, err := adapter.Client(context.Background(), "main"); err == nil {
		t.Fatal("client before init succeeded")
	}
	if err := adapter.Init(context.Background(), valid, socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); err == nil {
		t.Fatal("missing account succeeded")
	}
	if _, err := adapter.Client(context.Background(), "main"); err == nil {
		t.Fatal("missing access token secret succeeded")
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Init(context.Background(), valid); err == nil {
		t.Fatal("closed adapter reinitialized")
	}
}

func mutatedConfig(change func(*socialhub.AdapterConfig)) socialhub.AdapterConfig {
	value := testConfig("https://open.feishu.cn", TokenTenant, testChatID, testActorID, false)
	change(&value)
	return value
}

func TestScopePreflight(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	config := testConfig(server.URL, TokenTenant, testChatID, testActorID, false)
	config.Accounts[0].Approval.Scopes = []string{"im:message:readonly"}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config,
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://access-token": "token", "test://verification-token": "verify"}),
	); err != nil {
		t.Fatal(err)
	}
	common, err := adapter.Client(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	client := common.(*Client)
	if _, err := client.Send(context.Background(), SendRequest{ReceiveIDType: ReceiveChatID, ReceiveID: testChatID, MessageType: "text", Content: json.RawMessage(`{"text":"x"}`)}); err == nil {
		t.Fatal("missing write scope accepted")
	} else if platformErr := requireErrorCode(t, err, socialhub.CodeApprovalRequired); !slices.Contains(platformErr.RequiredScopes, "im:message") {
		t.Fatalf("required scopes=%v", platformErr.RequiredScopes)
	}
	if err := client.requireMessageRead("read"); err != nil {
		t.Fatalf("read scope rejected: %v", err)
	}
}
