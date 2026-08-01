package microsoftteams

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

const (
	testChatID    = "chat-1"
	testTeamID    = "team-1"
	testChannelID = "channel-1"
	testRootID    = "root-1"
	testReplyID   = "reply-1"
	testActorID   = "actor-1"
	testTenantID  = "tenant-1"
)

var testNow = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

type testSecrets map[string]string

func (s testSecrets) Resolve(_ context.Context, reference string) (string, error) {
	value := s[reference]
	if value == "" {
		return "", errors.New("missing test secret")
	}
	return value, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func testAccountSettings(cloud Cloud, kind TokenKind, defaultTarget bool) map[string]any {
	settings := map[string]any{
		"cloud": cloud, "token_kind": kind, "tenant_id": testTenantID, "actor_id": testActorID,
	}
	if defaultTarget {
		settings["default_chat_id"] = testChatID
	}
	return settings
}

func allTestScopes() []string {
	return []string{
		"ChannelMessage.Send", "ChannelMessage.Read.All", "ChannelMessage.ReadWrite",
		"ChatMessage.Send", "Chat.Read", "Chat.ReadWrite",
	}
}

func newTestAdapter(t *testing.T, handler http.Handler, cloud Cloud, kind TokenKind, defaultTarget, webhook bool, scopes []string) (*Adapter, *Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	settings := testAccountSettings(cloud, kind, defaultTarget)
	account := socialhub.AccountConfig{
		ID: "main", AccessTokenRef: "secret://token", Settings: settings,
		Approval: socialhub.ApprovalConfig{Scopes: scopes},
	}
	secrets := testSecrets{"secret://token": "access-token", "secret://state": "client-state"}
	if webhook {
		account.Webhook.SecretRef = "secret://state"
	}
	adapter := &Adapter{}
	err := adapter.Init(context.Background(), socialhub.AdapterConfig{
		Adapter: adapterName, Settings: map[string]any{"base_url": server.URL + "/v1.0"}, Accounts: []socialhub.AccountConfig{account},
	}, socialhub.WithSecretResolver(secrets), socialhub.WithHTTPClient(server.Client()), socialhub.WithClock(fixedClock{now: testNow}))
	if err != nil {
		server.Close()
		t.Fatalf("init adapter: %v", err)
	}
	opened, err := adapter.Client(context.Background(), "main")
	if err != nil {
		server.Close()
		t.Fatalf("open client: %v", err)
	}
	client := opened.(*Client)
	t.Cleanup(func() {
		_ = adapter.Close()
		server.Close()
	})
	return adapter, client, server
}

func TestAdapterLifecycleCapabilitiesAndRegistration(t *testing.T) {
	adapter, client, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenDelegated, true, true, allTestScopes())
	if adapter.Name() != adapterName || adapter.Metadata().APIVersion != "v1.0" || !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("adapter metadata or registration mismatch: %#v", adapter.Metadata())
	}
	if client.Platform() != "microsoft-teams" || client.Account() != "main" {
		t.Fatalf("client identity=%s/%s", client.Platform(), client.Account())
	}
	capabilities, err := client.Capabilities(context.Background())
	if err != nil || !capabilities.Has(socialhub.CapPublish) || !capabilities.Has(CapabilitySubscriptions) || capabilities[socialhub.CapMedia].Supported {
		t.Fatalf("capabilities=%#v error=%v", capabilities, err)
	}
	if _, ok := client.Publisher(); !ok {
		t.Fatal("publisher unavailable")
	}
	if _, ok := client.Fetcher(); !ok {
		t.Fatal("fetcher unavailable")
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
	if uploader, ok := client.MediaUploader(); ok || uploader != nil {
		t.Fatal("media uploader unexpectedly available")
	}
	if client.MessageWorkflow() == nil || client.ReactionWorkflow() == nil || client.SubscriptionWorkflow() == nil || client.Close() != nil {
		t.Fatal("typed workflow unavailable")
	}

	_, application, _ := newTestAdapter(t, http.NotFoundHandler(), CloudChina, TokenApplication, false, false, nil)
	if _, ok := application.Publisher(); ok {
		t.Fatal("application publisher unexpectedly available")
	}
	if _, ok := application.Reactor(); ok {
		t.Fatal("application reactor unexpectedly available")
	}
	if _, ok := application.WebhookHandler(); ok {
		t.Fatal("webhook unexpectedly available")
	}
	applicationCapabilities, _ := application.Capabilities(context.Background())
	if applicationCapabilities[socialhub.CapPublish].Supported || !applicationCapabilities[socialhub.CapFetch].Supported {
		t.Fatalf("application capabilities=%#v", applicationCapabilities)
	}
}

func TestAdapterValidationAndLifecycleErrors(t *testing.T) {
	validAccount := socialhub.AccountConfig{ID: "main", AccessTokenRef: "secret://token", Settings: testAccountSettings(CloudGlobal, TokenDelegated, true)}
	tests := []socialhub.AdapterConfig{
		{},
		{Adapter: "wrong", Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Settings: map[string]any{"base_url": "://bad"}, Accounts: []socialhub.AccountConfig{validAccount}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", Settings: testAccountSettings(CloudGlobal, TokenDelegated, true)}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "x", Settings: map[string]any{"cloud": "moon", "token_kind": TokenDelegated}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "x", Settings: map[string]any{"cloud": CloudGlobal, "token_kind": "bot"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "x", Settings: map[string]any{"cloud": CloudGlobal, "token_kind": TokenDelegated, "tenant_id": "bad/id"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "x", Settings: map[string]any{"cloud": CloudGlobal, "token_kind": TokenDelegated, "actor_id": "bad\n"}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "x", Settings: map[string]any{"cloud": CloudGlobal, "token_kind": TokenDelegated, "default_team_id": testTeamID}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "x", Settings: map[string]any{"cloud": CloudGlobal, "token_kind": TokenDelegated, "default_chat_id": testChatID, "default_team_id": testTeamID, "default_channel_id": testChannelID}}}},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "x", Settings: map[string]any{"cloud": CloudGlobal, "token_kind": TokenDelegated, "unknown": true}}}},
	}
	for index, config := range tests {
		adapter := &Adapter{}
		if err := adapter.Init(context.Background(), config, socialhub.WithSecretResolver(testSecrets{})); err == nil {
			t.Fatalf("invalid config %d accepted", index)
		}
	}

	adapter := &Adapter{}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client before init=%v", err)
	}
	if err := adapter.Init(context.Background(), socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{validAccount}}, socialhub.WithSecretResolver(testSecrets{"secret://token": "token"})); err != nil {
		t.Fatalf("valid init=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Init(context.Background(), socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{validAccount}}); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client after close=%v", err)
	}

	missingSecret := &Adapter{}
	if err := missingSecret.Init(context.Background(), socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{validAccount}}, socialhub.WithSecretResolver(testSecrets{})); err != nil {
		t.Fatal(err)
	}
	if _, err := missingSecret.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing token=%v", err)
	}
	longStateAccount := validAccount
	longStateAccount.Webhook.SecretRef = "secret://state"
	longState := &Adapter{}
	if err := longState.Init(context.Background(), socialhub.AdapterConfig{Adapter: adapterName, Accounts: []socialhub.AccountConfig{longStateAccount}}, socialhub.WithSecretResolver(testSecrets{"secret://token": "token", "secret://state": strings.Repeat("x", 129)})); err != nil {
		t.Fatal(err)
	}
	if _, err := longState.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long clientState=%v", err)
	}
}

func TestCloudEndpointsAndHelpers(t *testing.T) {
	endpoints := map[Cloud]string{
		CloudGlobal: "https://graph.microsoft.com/v1.0", CloudUSGov: "https://graph.microsoft.us/v1.0",
		CloudDoD: "https://dod-graph.microsoft.us/v1.0", CloudChina: "https://microsoftgraph.chinacloudapi.cn/v1.0",
	}
	for cloud, expected := range endpoints {
		if got := cloudBaseURL(cloud); got != expected {
			t.Fatalf("cloud %s=%s", cloud, got)
		}
	}
	if validEndpoint("ftp://example.com") || validEndpoint("https://user@example.com") || validOpaqueID("bad/id", 10) || !validOpaqueID("ok-id", 10) {
		t.Fatal("validation helper mismatch")
	}
	if target := defaultTarget(AccountSettings{}); target != nil {
		t.Fatalf("default target=%#v", target)
	}
	channel := defaultTarget(AccountSettings{DefaultTeamID: testTeamID, DefaultChannelID: testChannelID})
	if channel == nil || channel.Kind != TargetChannel {
		t.Fatalf("channel target=%#v", channel)
	}
}

func TestReferenceRoundTripsAndValidation(t *testing.T) {
	targets := []Target{
		{Kind: TargetChat, ChatID: "19:meeting_thread@thread.v2"},
		{Kind: TargetChannel, TeamID: testTeamID, ChannelID: testChannelID},
	}
	for _, target := range targets {
		conversation, err := ConversationRef(target)
		decoded, decodeErr := ParseConversationRef(conversation)
		if err != nil || decodeErr != nil || decoded != target {
			t.Fatalf("conversation %s => %#v errors=%v/%v", conversation, decoded, err, decodeErr)
		}
		for _, replyID := range []string{"", testReplyID} {
			ref := MessageRef{Target: target, RootID: testRootID, ReplyID: replyID}
			encoded, err := EncodeMessageRef(ref)
			decodedRef, decodeErr := ParseMessageRef(encoded)
			if err != nil || decodeErr != nil || decodedRef != ref {
				t.Fatalf("message %s => %#v errors=%v/%v", encoded, decodedRef, err, decodeErr)
			}
		}
	}
	invalidTargets := []Target{{}, {Kind: TargetChat}, {Kind: TargetChat, ChatID: testChatID, TeamID: testTeamID}, {Kind: TargetChannel, TeamID: testTeamID}, {Kind: "meeting", ChatID: testChatID}}
	for _, target := range invalidTargets {
		if _, err := ConversationRef(target); err == nil {
			t.Fatalf("invalid target accepted: %#v", target)
		}
	}
	for _, value := range []string{"", "chat", "chat~%%%", "channel~dGVhbQ", "channel~%%%~Y2hhbm5lbA", "chat~Y2hhdA~cm9vdA~cmVwbHk~ZXh0cmE"} {
		if _, err := ParseConversationRef(value); err == nil {
			t.Fatalf("invalid conversation accepted: %q", value)
		}
		if _, err := ParseMessageRef(value); err == nil {
			t.Fatalf("invalid message accepted: %q", value)
		}
	}
	if _, err := EncodeMessageRef(MessageRef{Target: targets[0]}); err == nil {
		t.Fatal("empty root accepted")
	}
}
