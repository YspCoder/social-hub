package kakao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

type testResolver map[string]string

func (resolver testResolver) Resolve(_ context.Context, reference string) (string, error) {
	value, found := resolver[reference]
	if !found {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

var testNow = time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)

func testConfig(serverURL string, friendApproved, commonMessenger bool) socialhub.AdapterConfig {
	settings := map[string]any{"user_id": "123456789", "friend_message_approved": friendApproved}
	if commonMessenger {
		settings["default_link_url"] = "https://app.example.test/message"
	}
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Settings: map[string]any{
			"base_url": serverURL, "auth_url": serverURL + "/oauth/authorize", "token_url": serverURL + "/oauth/token",
		},
		Accounts: []socialhub.AccountConfig{{
			ID: "main", ClientID: "rest-api-key", SecretRef: "test://client-secret", AccessTokenRef: "test://access-token",
			Settings: settings, Approval: socialhub.ApprovalConfig{Scopes: []string{"profile_nickname", "talk_message", "friends"}},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, friendApproved, commonMessenger bool) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, friendApproved, commonMessenger),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://access-token": "access-token", "test://client-secret": "client-secret"}),
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

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, true, true)
	if adapter.Name() != adapterName || client.Platform() != "kakao" || client.Account() != "main" {
		t.Fatalf("identity=%s %s/%s", adapter.Name(), client.Platform(), client.Account())
	}
	metadata := adapter.Metadata()
	if metadata.Product != productName || metadata.APIVersion != apiVersion || metadata.DocURL != docURL || metadata.VerifiedAt.IsZero() {
		t.Fatalf("metadata=%#v", metadata)
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []socialhub.Capability{socialhub.CapFetch, socialhub.CapMessage, CapabilityTalkFriends, CapabilityTemplates} {
		if !capabilities.Has(capability) || capabilities[capability].DocURL == "" {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	for _, capability := range []socialhub.Capability{socialhub.CapPublish, socialhub.CapMedia, socialhub.CapReact, socialhub.CapWebhook} {
		if capabilities.Has(capability) {
			t.Fatalf("unsupported capability %s=%#v", capability, capabilities[capability])
		}
	}
	if _, ok := client.Publisher(); ok {
		t.Fatal("publisher should be unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher should be available")
	}
	if _, ok := client.MediaUploader(); ok {
		t.Fatal("media uploader should be unavailable")
	}
	if _, ok := client.Reactor(); ok {
		t.Fatal("reactor should be unavailable")
	}
	if _, ok := client.Messenger(); !ok {
		t.Fatal("messenger should be available")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if client.UserWorkflow() == nil || client.FriendWorkflow() == nil || client.MessageWorkflow() == nil {
		t.Fatal("typed workflows should be available")
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
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("OAuth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, true, true)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestOptionalCommonMessengerAndFriendApproval(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, false)
	if _, ok := client.Messenger(); ok {
		t.Fatal("common Messenger should require default_link_url")
	}
	capabilities, _ := client.Capabilities(context.Background())
	if capabilities.Has(socialhub.CapMessage) || capabilities[CapabilityTalkFriends].Approval != socialhub.ApprovalRequired || capabilities.Has(CapabilityTalkFriends) {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if _, err := client.ListFriends(context.Background(), ListFriendsRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("unapproved friend list=%v", err)
	}
	if _, err := client.SendDefault(context.Background(), DefaultMessageRequest{
		Target: MessageTargetFriends, ReceiverUUIDs: []string{"friend-1"},
		Template: TextTemplate{Text: "hello", Link: Link{WebURL: "https://app.example.test"}},
	}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("unapproved friend send=%v", err)
	}
}

func TestAdapterValidationAndCredentialResolution(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	valid := testConfig(server.URL, true, true)
	invalid := []socialhub.AdapterConfig{
		{Adapter: adapterName},
		{Adapter: "kakao", Accounts: valid.Accounts},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "https://user:pass@example.test"}, Accounts: valid.Accounts},
		{Adapter: adapterName, Settings: map[string]any{"auth_url": "relative"}, Accounts: valid.Accounts},
		{Adapter: adapterName, Settings: map[string]any{"unknown": true}, Accounts: valid.Accounts},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", Settings: map[string]any{"user_id": "1"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", SecretRef: "secret", AccessTokenRef: "token", Settings: map[string]any{"user_id": "1"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Webhook: socialhub.WebhookConfig{SecretRef: "secret"}, Settings: map[string]any{"user_id": "1"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"user_id": "0"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"user_id": "1", "default_link_url": "file:///tmp"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: map[string]any{"user_id": "1", "unknown": true}}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), valid, socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing access token=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing client secret=%v", err)
	}

	withoutOAuth := valid
	withoutOAuth.Accounts = append([]socialhub.AccountConfig(nil), valid.Accounts...)
	withoutOAuth.Accounts[0].ClientID, withoutOAuth.Accounts[0].SecretRef = "", ""
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), withoutOAuth); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.OAuth(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) || !strings.Contains(err.Error(), "invalid_argument") {
		t.Fatalf("OAuth without client ID=%v", err)
	}
}
