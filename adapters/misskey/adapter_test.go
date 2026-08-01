package misskey

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

func allTestPermissions() []string {
	return []string{"read:account", "write:notes", "read:drive", "write:drive", "write:reactions"}
}

func testConfig(instanceURL string, permissions []string) socialhub.AdapterConfig {
	return socialhub.AdapterConfig{
		Adapter: adapterName, Product: productName,
		Accounts: []socialhub.AccountConfig{{
			ID: "main", AccessTokenRef: "test://access-token",
			Settings: map[string]any{
				"instance_url": instanceURL, "user_id": "user-1", "default_reaction": ":thumbsup:",
			},
			Approval: socialhub.ApprovalConfig{Scopes: permissions},
		}},
	}
}

func newTestClient(t *testing.T, server *httptest.Server, permissions []string) (*Adapter, *Client) {
	t.Helper()
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), testConfig(server.URL, permissions),
		socialhub.WithHTTPClient(server.Client()),
		socialhub.WithSecretResolver(testResolver{"test://access-token": "access-token"}),
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

func testUser(id string) map[string]any {
	return map[string]any{
		"id": id, "name": "Alice", "username": "alice", "host": nil,
		"avatarUrl": "https://cdn.example.test/avatar.png", "url": "https://social.example.test/@alice",
		"createdAt": testNow.Add(-24 * time.Hour), "description": "profile", "location": "Tokyo",
		"lang": "ja", "bannerUrl": "https://cdn.example.test/banner.png", "onlineStatus": "online",
		"followersCount": 3, "followingCount": 4, "notesCount": 5,
	}
}

func testDriveFile(id, mimeType string) map[string]any {
	return map[string]any{
		"id": id, "createdAt": testNow, "name": "media.bin", "type": mimeType,
		"md5": "abc", "size": 10, "isSensitive": false, "blurhash": "hash",
		"properties": map[string]any{"width": 640, "height": 480},
		"url":        "https://cdn.example.test/" + id, "thumbnailUrl": "https://cdn.example.test/thumb.jpg",
		"comment": "alt", "folderId": "folder-1", "userId": "user-1",
	}
}

func testNote(id, text string) map[string]any {
	return map[string]any{
		"id": id, "createdAt": testNow, "text": text, "cw": nil, "userId": "user-1",
		"user": testUser("user-1"), "replyId": nil, "renoteId": nil, "visibility": "public",
		"visibleUserIds": []string{}, "files": []any{}, "tags": []string{"go"}, "poll": nil,
		"channelId": nil, "localOnly": false, "reactionAcceptance": nil,
		"reactions": map[string]int{":thumbsup:": 2}, "reactionCount": 2, "renoteCount": 3,
		"repliesCount": 4, "uri": "https://social.example.test/notes/" + id,
		"url": "https://social.example.test/notes/" + id, "myReaction": ":thumbsup:",
	}
}

func TestAdapterRegistrationCapabilitiesAndLifecycle(t *testing.T) {
	if !slices.Contains(socialhub.Adapters(), adapterName) {
		t.Fatalf("registered adapters=%v", socialhub.Adapters())
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	adapter, client := newTestClient(t, server, allTestPermissions())
	if adapter.Name() != adapterName || client.Platform() != "misskey" || client.Account() != "main" {
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
	for _, capability := range []socialhub.Capability{
		socialhub.CapPublish, socialhub.CapFetch, socialhub.CapMedia, socialhub.CapReact,
		CapabilityHomeTimeline, CapabilityMiAuth,
	} {
		if !capabilities.Has(capability) {
			t.Fatalf("capability %s=%#v", capability, capabilities[capability])
		}
	}
	if capabilities.Has(socialhub.CapMessage) || capabilities.Has(socialhub.CapWebhook) {
		t.Fatalf("unsupported capabilities=%#v", capabilities)
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
	if _, ok := client.Messenger(); ok {
		t.Fatal("messenger should be unavailable")
	}
	if _, ok := client.WebhookHandler(); ok {
		t.Fatal("webhook should be unavailable")
	}
	if client.NoteWorkflow() == nil || client.TimelineWorkflow() == nil || client.DriveWorkflow() == nil || client.InstanceWorkflow() == nil {
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
	if _, err := adapter.MiAuth("main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("MiAuth after close=%v", err)
	}
	if err := adapter.Init(context.Background(), testConfig(server.URL, nil)); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("init after close=%v", err)
	}
}

func TestAdapterValidationCredentialsAndPermissions(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	validAccount := func(settings map[string]any) []socialhub.AccountConfig {
		return []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Settings: settings}}
	}
	validSettings := func() map[string]any { return map[string]any{"instance_url": server.URL, "user_id": "user-1"} }
	invalid := []socialhub.AdapterConfig{
		{Adapter: adapterName},
		{Adapter: "misskey", Accounts: validAccount(validSettings())},
		{Adapter: adapterName, Settings: map[string]any{"global": true}, Accounts: validAccount(validSettings())},
		{Adapter: adapterName, Accounts: validAccount(map[string]any{"instance_url": "/relative"})},
		{Adapter: adapterName, Accounts: validAccount(map[string]any{"instance_url": server.URL + "/path"})},
		{Adapter: adapterName, Accounts: validAccount(map[string]any{"instance_url": server.URL, "user_id": " bad"})},
		{Adapter: adapterName, Accounts: validAccount(map[string]any{"instance_url": server.URL, "default_reaction": "bad\n"})},
		{Adapter: adapterName, Accounts: validAccount(map[string]any{"instance_url": server.URL, "unknown": true})},
		{Adapter: adapterName, Accounts: []socialhub.AccountConfig{{ID: "main", AccessTokenRef: "token", Webhook: socialhub.WebhookConfig{SecretRef: "secret"}, Settings: validSettings()}}},
	}
	for index, config := range invalid {
		if err := (&Adapter{}).Init(context.Background(), config); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid config %d=%v", index, err)
		}
	}
	beforeInit := &Adapter{}
	if _, err := beforeInit.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("client before init=%v", err)
	}
	if _, err := beforeInit.MiAuth("main"); !errors.Is(err, socialhub.ErrConflict) {
		t.Fatalf("MiAuth before init=%v", err)
	}

	config := testConfig(server.URL, []string{"read:account"})
	adapter := &Adapter{}
	if err := adapter.Init(context.Background(), config, socialhub.WithSecretResolver(testResolver{})); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("missing secret=%v", err)
	}
	if _, err := adapter.Client(context.Background(), "missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing account=%v", err)
	}
	if _, err := adapter.MiAuth("missing"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing MiAuth account=%v", err)
	}

	config.Accounts[0].AccessTokenRef = ""
	adapter = &Adapter{}
	if err := adapter.Init(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Client(context.Background(), "main"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing token ref=%v", err)
	}

	_, restricted := newTestClient(t, server, []string{"read:account"})
	capabilities, _ := restricted.Capabilities(context.Background())
	if capabilities[socialhub.CapPublish].Approval != socialhub.ApprovalRequired || capabilities.Has(socialhub.CapPublish) {
		t.Fatalf("publish capability=%#v", capabilities[socialhub.CapPublish])
	}
	text := "hello"
	if _, err := restricted.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("scope gate=%v", err)
	}
}
