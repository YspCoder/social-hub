package kick

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestPublicAPIWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Accept") != "application/json" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case "GET /public/v1/users":
			if !reflect.DeepEqual(request.URL.Query()["id"], []string{"1", "2"}) {
				http.Error(writer, "bad user query", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, map[string]any{"data": []map[string]any{{"user_id": 1, "name": "Alice", "profile_picture": "https://img.test/a"}}, "message": "OK"})
		case "GET /public/v1/channels":
			if request.URL.Query().Get("broadcaster_user_id") != "123" {
				http.Error(writer, "bad channel query", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, map[string]any{"data": []map[string]any{{
				"broadcaster_user_id": 123, "slug": "streamer", "stream_title": "Live",
				"category": map[string]any{"id": 9, "name": "Science", "thumbnail": "https://img.test/c"},
				"stream":   map[string]any{"is_live": true, "start_time": "2026-08-02T09:00:00Z", "viewer_count": 42},
			}}})
		case "PATCH /public/v1/channels":
			var body struct {
				StreamTitle string   `json:"stream_title"`
				CategoryID  int64    `json:"category_id"`
				CustomTags  []string `json:"custom_tags"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.StreamTitle != "New title" || body.CategoryID != 9 || !reflect.DeepEqual(body.CustomTags, []string{"go", "sdk"}) {
				http.Error(writer, "bad channel body", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		case "GET /public/v2/livestreams":
			query := request.URL.Query()
			if query.Get("cursor") != "cursor-1" || query.Get("limit") != "50" || !reflect.DeepEqual(query["category_id"], []string{"9", "10"}) || !reflect.DeepEqual(query["language_code"], []string{"en", "zh-Hans"}) {
				http.Error(writer, "bad live query", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, livePageFixture("next-live"))
		case "GET /public/v1/users/livestreams":
			if !reflect.DeepEqual(request.URL.Query()["user_id"], []string{"123", "456"}) {
				http.Error(writer, "bad user live query", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, map[string]any{"data": liveItemsFixture()})
		case "GET /public/v2/categories":
			query := request.URL.Query()
			if query.Get("name") != "Science,Music" || query.Get("tag") != "edu" || query.Get("id") != "9,10" || query.Get("cursor") != "abcd" || query.Get("limit") != "25" {
				http.Error(writer, "bad category query", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, map[string]any{
				"data":       []map[string]any{{"id": 9, "name": "Science", "tags": []string{"edu"}, "thumbnail": "https://img.test/c"}},
				"pagination": map[string]any{"next_cursor": "next-category"},
			})
		case "POST /public/v1/chat":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["content"] != "hello" || body["type"] != "user" || body["broadcaster_user_id"] != float64(123) || body["reply_to_message_id"] != "parent-1" {
				http.Error(writer, "bad chat body", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, map[string]any{"data": map[string]any{"is_sent": true, "message_id": "message-1"}})
		case "DELETE /public/v1/chat/message-1":
			writer.WriteHeader(http.StatusNoContent)
		case "GET /public/v1/events/subscriptions":
			if request.URL.Query().Get("broadcaster_user_id") != "123" {
				http.Error(writer, "bad subscription query", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, map[string]any{"data": []map[string]any{{
				"app_id": "app-1", "broadcaster_user_id": 123, "created_at": "2026-08-02T08:00:00Z",
				"updated_at": "2026-08-02T08:00:00Z", "event": "channel.followed", "id": "sub-1", "method": "webhook", "version": 1,
			}}})
		case "POST /public/v1/events/subscriptions":
			var body struct {
				BroadcasterUserID *int64                     `json:"broadcaster_user_id"`
				Events            []EventSubscriptionRequest `json:"events"`
				Method            string                     `json:"method"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.BroadcasterUserID != nil || body.Method != "webhook" || len(body.Events) != 1 || body.Events[0].Name != "channel.followed" {
				http.Error(writer, "bad subscription body", http.StatusBadRequest)
				return
			}
			writeJSON(t, writer, map[string]any{"data": []map[string]any{{"name": "channel.followed", "version": 1, "subscription_id": "sub-1"}}})
		case "DELETE /public/v1/events/subscriptions":
			if !reflect.DeepEqual(request.URL.Query()["id"], []string{"sub-1", "sub-2"}) {
				http.Error(writer, "bad subscription delete", http.StatusBadRequest)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		case "GET /public/v1/public-key":
			writeJSON(t, writer, map[string]any{"data": map[string]any{"public_key": defaultWebhookPublicKey}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	scopes := []string{"user:read", "channel:read", "channel:write", "chat:write", "moderation:chat_message:manage", "events:subscribe"}
	_, client := newTestClient(t, server, "user", scopes)
	users, err := client.ListUsers(context.Background(), []string{"1", "2"})
	if err != nil || len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("users: %#v %v", users, err)
	}
	channels, err := client.ListChannels(context.Background(), ChannelListRequest{})
	if err != nil || len(channels) != 1 || channels[0].Stream.ViewerCount != 42 {
		t.Fatalf("channels: %#v %v", channels, err)
	}
	title, category, tags := "New title", int64(9), []string{"go", "sdk"}
	if err := client.UpdateChannel(context.Background(), UpdateChannelRequest{StreamTitle: &title, CategoryID: &category, CustomTags: &tags}); err != nil {
		t.Fatalf("update channel: %v", err)
	}
	live, err := client.ListLivestreams(context.Background(), LivestreamListRequest{
		CategoryIDs: []string{"9", "10"}, LanguageCodes: []string{"en", "zh-Hans"}, Cursor: "cursor-1", Limit: 50,
	})
	if err != nil || len(live.Items) != 1 || live.NextCursor == nil || *live.NextCursor != "next-live" || !live.HasMore {
		t.Fatalf("livestreams: %#v %v", live, err)
	}
	userLive, err := client.ListUserLivestreams(context.Background(), []string{"123", "456"})
	if err != nil || len(userLive) != 1 || userLive[0].ID != "live-1" {
		t.Fatalf("user livestreams: %#v %v", userLive, err)
	}
	categories, err := client.ListCategories(context.Background(), CategoryListRequest{
		Names: []string{"Science", "Music"}, Tags: []string{"edu"}, IDs: []string{"9", "10"}, Cursor: "abcd", Limit: 25,
	})
	if err != nil || len(categories.Items) != 1 || categories.NextCursor == nil || *categories.NextCursor != "next-category" {
		t.Fatalf("categories: %#v %v", categories, err)
	}
	chat, err := client.SendChat(context.Background(), SendChatRequest{Content: "hello", Type: "user", ReplyToMessageID: "parent-1"})
	if err != nil || chat.MessageID != "message-1" {
		t.Fatalf("send chat: %#v %v", chat, err)
	}
	if err := client.DeleteChat(context.Background(), "message-1"); err != nil {
		t.Fatalf("delete chat: %v", err)
	}
	subscriptions, err := client.ListSubscriptions(context.Background(), "123")
	if err != nil || len(subscriptions) != 1 || subscriptions[0].ID != "sub-1" {
		t.Fatalf("subscriptions: %#v %v", subscriptions, err)
	}
	created, err := client.CreateSubscriptions(context.Background(), CreateSubscriptionsRequest{Events: []EventSubscriptionRequest{{Name: "channel.followed", Version: 1}}})
	if err != nil || len(created) != 1 || created[0].SubscriptionID != "sub-1" {
		t.Fatalf("create subscriptions: %#v %v", created, err)
	}
	if err := client.DeleteSubscriptions(context.Background(), []string{"sub-1", "sub-2"}); err != nil {
		t.Fatalf("delete subscriptions: %v", err)
	}
	publicKey, err := client.FetchWebhookPublicKey(context.Background())
	if err != nil || publicKey != defaultWebhookPublicKey {
		t.Fatalf("public key: %q %v", publicKey, err)
	}
}

func liveItemsFixture() []map[string]any {
	return []map[string]any{{
		"id": "live-1", "broadcaster_user": map[string]any{"id": 123, "username": "streamer", "profile_picture": "https://img.test/u"},
		"category": map[string]any{"id": 9, "name": "Science", "thumbnail": "https://img.test/c"},
		"channel":  map[string]any{"slug": "streamer"}, "has_mature_content": false, "language_code": "en",
		"started_at": "2026-08-02T09:00:00Z", "tags": []string{"go"}, "thumbnail": "https://img.test/live", "title": "Live", "viewer_count": 42,
	}}
}

func livePageFixture(cursor string) map[string]any {
	return map[string]any{"data": liveItemsFixture(), "pagination": map[string]any{"next_cursor": cursor}}
}

func writeJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
}

func TestWorkflowValidationAndApproval(t *testing.T) {
	user := &Client{tokenType: "user", broadcasterUserID: "123", scopes: []string{"user:read", "chat:write", "moderation:chat_message:manage"}}
	app := &Client{tokenType: "app"}
	assertInvalid := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	_, err := user.ListUsers(context.Background(), []string{"bad"})
	assertInvalid("user ID", err)
	_, err = app.ListChannels(context.Background(), ChannelListRequest{BroadcasterUserIDs: []string{"1"}, Slugs: []string{"slug"}})
	assertInvalid("mixed channels", err)
	_, err = app.ListChannels(context.Background(), ChannelListRequest{Slugs: []string{"bad/slug"}})
	assertInvalid("bad slug", err)
	if err := app.UpdateChannel(context.Background(), UpdateChannelRequest{}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("app update: %v", err)
	}
	assertInvalid("empty update", (&Client{tokenType: "user"}).UpdateChannel(context.Background(), UpdateChannelRequest{}))
	emptyTitle := " "
	assertInvalid("empty title", (&Client{tokenType: "user"}).UpdateChannel(context.Background(), UpdateChannelRequest{StreamTitle: &emptyTitle}))
	badCategory := int64(0)
	assertInvalid("category", (&Client{tokenType: "user"}).UpdateChannel(context.Background(), UpdateChannelRequest{CategoryID: &badCategory}))
	manyTags := make([]string, 11)
	assertInvalid("tags", (&Client{tokenType: "user"}).UpdateChannel(context.Background(), UpdateChannelRequest{CustomTags: &manyTags}))
	badTags := []string{""}
	assertInvalid("bad tag", (&Client{tokenType: "user"}).UpdateChannel(context.Background(), UpdateChannelRequest{CustomTags: &badTags}))

	_, err = app.ListLivestreams(context.Background(), LivestreamListRequest{Limit: 1001})
	assertInvalid("live limit", err)
	_, err = app.ListLivestreams(context.Background(), LivestreamListRequest{CategoryIDs: make([]string, 26)})
	assertInvalid("live filters", err)
	_, err = app.ListLivestreams(context.Background(), LivestreamListRequest{Cursor: " bad"})
	assertInvalid("live cursor", err)
	_, err = app.ListLivestreams(context.Background(), LivestreamListRequest{CategoryIDs: []string{"bad"}})
	assertInvalid("live category", err)
	_, err = app.ListLivestreams(context.Background(), LivestreamListRequest{LanguageCodes: []string{""}})
	assertInvalid("live language", err)
	_, err = app.ListUserLivestreams(context.Background(), nil)
	assertInvalid("user live empty", err)
	_, err = app.ListUserLivestreams(context.Background(), make([]string, 101))
	assertInvalid("user live max", err)

	_, err = app.ListCategories(context.Background(), CategoryListRequest{Limit: -1})
	assertInvalid("category limit", err)
	_, err = app.ListCategories(context.Background(), CategoryListRequest{Cursor: "x"})
	assertInvalid("category cursor", err)
	_, err = app.ListCategories(context.Background(), CategoryListRequest{Names: []string{"bad,name"}})
	assertInvalid("category name", err)
	_, err = app.ListCategories(context.Background(), CategoryListRequest{Tags: []string{""}})
	assertInvalid("category tag", err)
	_, err = app.ListCategories(context.Background(), CategoryListRequest{IDs: []string{"0"}})
	assertInvalid("category ID", err)

	_, err = app.SendChat(context.Background(), SendChatRequest{Content: "x", Type: "bot"})
	if !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("app chat: %v", err)
	}
	_, err = user.SendChat(context.Background(), SendChatRequest{Content: "", Type: "user"})
	assertInvalid("empty chat", err)
	_, err = user.SendChat(context.Background(), SendChatRequest{Content: strings.Repeat("x", 501), Type: "user"})
	assertInvalid("long chat", err)
	_, err = user.SendChat(context.Background(), SendChatRequest{Content: "x", Type: "bad"})
	assertInvalid("chat type", err)
	_, err = (&Client{tokenType: "user", scopes: []string{"chat:write"}}).SendChat(context.Background(), SendChatRequest{Content: "x", Type: "user"})
	assertInvalid("chat broadcaster", err)
	_, err = user.SendChat(context.Background(), SendChatRequest{Content: "x", Type: "user", ReplyToMessageID: "bad/id"})
	assertInvalid("chat reply", err)
	assertInvalid("delete chat", user.DeleteChat(context.Background(), "bad/id"))

	_, err = app.ListSubscriptions(context.Background(), "bad")
	assertInvalid("subscription broadcaster", err)
	_, err = app.CreateSubscriptions(context.Background(), CreateSubscriptionsRequest{})
	assertInvalid("empty subscriptions", err)
	_, err = app.CreateSubscriptions(context.Background(), CreateSubscriptionsRequest{Events: []EventSubscriptionRequest{{Name: "channel.followed", Version: 1}}})
	assertInvalid("app subscription broadcaster", err)
	_, err = (&Client{tokenType: "app", broadcasterUserID: "123"}).CreateSubscriptions(context.Background(), CreateSubscriptionsRequest{Events: []EventSubscriptionRequest{{Name: "unknown", Version: 1}}})
	assertInvalid("subscription event", err)
	_, err = (&Client{tokenType: "app", broadcasterUserID: "123"}).CreateSubscriptions(context.Background(), CreateSubscriptionsRequest{Events: []EventSubscriptionRequest{{Name: "channel.followed", Version: 1}, {Name: "channel.followed", Version: 1}}})
	assertInvalid("duplicate subscription", err)
	assertInvalid("empty subscription delete", app.DeleteSubscriptions(context.Background(), nil))
	assertInvalid("bad subscription delete", app.DeleteSubscriptions(context.Background(), []string{""}))

	noScope := &Client{tokenType: "user", scopes: []string{"user:read"}}
	if _, err := noScope.SendChat(context.Background(), SendChatRequest{Content: "x", Type: "bot"}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing chat scope: %v", err)
	}
	if _, err := noScope.CreateSubscriptions(context.Background(), CreateSubscriptionsRequest{Events: []EventSubscriptionRequest{{Name: "channel.followed", Version: 1}}}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing subscription scope: %v", err)
	}
}
