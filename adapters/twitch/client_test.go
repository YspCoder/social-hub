package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestFetchAndMessageContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer user-token" || request.Header.Get("Client-Id") != "twitch-client" {
			http.Error(writer, "bad auth", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/users":
			if request.URL.Query().Get("id") != "user-1" {
				http.Error(writer, "bad user", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []map[string]any{{
				"id": "user-1", "login": "alice", "display_name": "Alice", "broadcaster_type": "partner",
				"description": "creator", "profile_image_url": "https://cdn.test/avatar.png", "created_at": "2020-01-01T00:00:00Z",
			}}})
		case "/videos":
			query := request.URL.Query()
			if query.Get("id") == "video-1" || query.Get("user_id") == "user-1" {
				writeTestJSON(t, writer, map[string]any{
					"data": []map[string]any{{
						"id": "video-1", "stream_id": "stream-1", "user_id": "user-1", "user_login": "alice", "user_name": "Alice",
						"title": "Build in Go", "description": "description", "created_at": "2026-07-31T10:00:00Z",
						"published_at": "2026-07-31T10:01:00Z", "url": "https://www.twitch.tv/videos/video-1",
						"thumbnail_url": "https://cdn.test/thumb.jpg", "viewable": "public", "view_count": 42,
						"language": "en", "type": "archive", "duration": "1h2m3s",
					}}, "pagination": map[string]string{"cursor": "next-video"},
				})
				return
			}
			http.Error(writer, "not found", http.StatusNotFound)
		case "/chat/messages":
			if request.Method != http.MethodPost {
				http.Error(writer, "method", http.StatusMethodNotAllowed)
				return
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["broadcaster_id"] != "channel-1" || body["sender_id"] != "user-1" || body["message"] != "hello" || body["reply_parent_message_id"] != "parent-1" {
				http.Error(writer, "bad message", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"data": []map[string]any{{"message_id": "message-1", "is_sent": true}}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := newTestClient(t, server, []string{"user:write:chat", "clips:edit"})
	user, err := client.GetUser(context.Background(), "me")
	if err != nil || user.ID != "user-1" || user.Username == nil || *user.Username != "alice" || user.ProfileURL == nil || *user.ProfileURL != "https://www.twitch.tv/alice" || len(user.Extensions) != 1 {
		t.Fatalf("get user: %#v %v", user, err)
	}
	post, err := client.GetPost(context.Background(), "video-1")
	if err != nil || post.ID != "video-1" || post.Text == nil || *post.Text != "Build in Go" || post.AuthorID == nil || *post.AuthorID != "user-1" || post.CreatedAt == nil || post.CreatedAt.Format(time.RFC3339) != "2026-07-31T10:01:00Z" || len(post.Metrics) != 1 || post.Metrics[0].Value != 42 || !post.Metrics[0].AsOf.Equal(testNow) {
		t.Fatalf("get post: %#v %v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "me", Cursor: "cursor-1", MaxResults: 250})
	if err != nil || len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "next-video" || !page.HasMore {
		t.Fatalf("list posts: %#v %v", page, err)
	}
	text, reply := "hello", "parent-1"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "channel-1", Text: &text, ReplyToID: &reply})
	if err != nil || message.ID != "message-1" || message.SenderID == nil || *message.SenderID != "user-1" || message.Direction != socialhub.DirectionOutbound || message.SentAt == nil || !message.SentAt.Equal(testNow) {
		t.Fatalf("send message: %#v %v", message, err)
	}
	if _, err := client.GetMessage(context.Background(), "message-1"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message: %v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "video-1"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments: %v", err)
	}
}

func TestFetchAndMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, []string{"clips:edit"})
	if _, err := client.GetPost(context.Background(), " "); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty post: %v", err)
	}
	client.userID = ""
	if _, err := client.GetUser(context.Background(), "me"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing user: %v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing list user: %v", err)
	}
	client.userID = "user-1"
	start := testNow.Add(-time.Hour)
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &start}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time range: %v", err)
	}
	text, blank, tooLong, reply := "hello", " ", strings.Repeat("x", 501), " "
	requests := []socialhub.SendMessageRequest{
		{}, {ConversationID: "channel", Text: &blank}, {ConversationID: "channel", Text: &tooLong},
		{ConversationID: "channel", Text: &text, RecipientIDs: []string{"user"}},
		{ConversationID: "channel", Text: &text, MediaIDs: []string{"media"}},
		{ConversationID: "channel", Text: &text, ReplyToID: &reply},
	}
	for index, request := range requests {
		if _, err := client.SendMessage(context.Background(), request); err == nil {
			t.Fatalf("message case %d accepted", index)
		}
	}
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "channel", Text: &text}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing scope: %v", err)
	}
	client.scopes = nil
	client.userID = ""
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "channel", Text: &text}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing configured user: %v", err)
	}
}

func TestDroppedChatMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeTestJSON(t, writer, map[string]any{"data": []map[string]any{{
			"message_id": "message-1", "is_sent": false,
			"drop_reason": map[string]string{"code": "automod_held", "message": "held by AutoMod"},
		}}})
	}))
	defer server.Close()
	client := newTestClient(t, server, []string{"user:write:chat"})
	text := "hello"
	_, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "channel", Text: &text})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.PlatformCode != "automod_held" {
		t.Fatalf("drop error: %#v", err)
	}
}
