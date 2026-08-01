package slack

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

func requireSlackRequest(t *testing.T, writer http.ResponseWriter, request *http.Request, method string) map[string]any {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.Path != "/api/"+method || request.URL.RawQuery != "" {
		http.Error(writer, "bad request target", http.StatusBadRequest)
		t.Errorf("request=%s %s", request.Method, request.URL.String())
		return nil
	}
	if request.Header.Get("Authorization") != "Bearer xoxb-test-token" || request.Header.Get("Content-Type") != "application/json; charset=utf-8" || request.Header.Get("Idempotency-Key") != "" {
		http.Error(writer, "bad request headers", http.StatusUnauthorized)
		t.Errorf("headers=%v", request.Header)
		return nil
	}
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		http.Error(writer, "bad JSON", http.StatusBadRequest)
		t.Errorf("decode request: %v", err)
		return nil
	}
	return body
}

func TestChatPublisherAndMessengerContracts(t *testing.T) {
	postCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/api/")
		body := requireSlackRequest(t, writer, request, method)
		if body == nil {
			return
		}
		switch method {
		case "chat.postMessage":
			postCalls++
			if body["channel"] != testChannelID || request.Header.Get("X-Request-ID") != "request-id" {
				t.Errorf("post body=%v headers=%v", body, request.Header)
			}
			timestamp := testTimestamp
			threadTS := ""
			if postCalls == 2 {
				timestamp, threadTS = "1785571200.000002", testTimestamp
				if body["thread_ts"] != testTimestamp {
					t.Errorf("thread post body=%v", body)
				}
			} else if postCalls == 3 {
				timestamp = "1785571200.000003"
			}
			message := map[string]any{
				"type": "message", "user": testActorID, "text": body["text"], "ts": timestamp,
				"thread_ts": threadTS, "reply_count": 2,
				"reactions": []any{map[string]any{"name": "thumbsup", "count": 3}},
				"files":     []any{map[string]any{"id": testFileID, "name": "image.png", "mimetype": "image/png", "size": 10, "url_private": "https://files.test/image.png", "original_w": 640, "original_h": 480}},
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "channel": testChannelID, "ts": timestamp, "message": message})
		case "chat.update":
			if body["channel"] != testChannelID || body["ts"] != testTimestamp || body["text"] != "updated" {
				t.Errorf("update body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "channel": testChannelID, "ts": testTimestamp, "text": "updated"})
		case "chat.delete":
			if body["channel"] != testChannelID || body["ts"] != testTimestamp {
				t.Errorf("delete body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "channel": testChannelID, "ts": testTimestamp})
		case "conversations.history":
			timestamp, _ := body["oldest"].(string)
			message := map[string]any{"type": "message", "user": testActorID, "text": "history", "ts": timestamp}
			writeTestJSON(t, writer, map[string]any{"ok": true, "messages": []any{message}, "has_more": false})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, false, allTestScopes())

	text := "hello Slack"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text}, socialhub.WithRequestID("request-id"))
	if err != nil || post.ID != testChannelID+":"+testTimestamp || post.Text == nil || *post.Text != text || post.AuthorID == nil || *post.AuthorID != testActorID || post.CreatedAt == nil || post.Visibility == nil || *post.Visibility != "workspace" || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeImage || len(post.Metrics) != 2 || len(post.Extensions) != 1 {
		t.Fatalf("published post=%#v error=%v", post, err)
	}
	reply, err := client.PostMessage(context.Background(), PostMessageRequest{ChannelID: testChannelID, Text: "reply", ThreadPostID: post.ID}, socialhub.WithRequestID("request-id"))
	if err != nil || reply.ID != testChannelID+":1785571200.000002" || len(reply.Relations) != 1 || reply.Relations[0].PostID != post.ID {
		t.Fatalf("reply=%#v error=%v", reply, err)
	}
	updated, err := client.UpdateMessage(context.Background(), UpdateMessageRequest{PostID: post.ID, Text: "updated"})
	if err != nil || updated.ID != post.ID || updated.Text == nil || *updated.Text != "updated" {
		t.Fatalf("updated=%#v error=%v", updated, err)
	}
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: testChannelID, Text: &text}, socialhub.WithRequestID("request-id"))
	if err != nil || message.ID != testChannelID+":1785571200.000003" || message.ConversationID != testChannelID || message.Direction != socialhub.DirectionOutbound || len(message.RecipientIDs) != 1 || message.SentAt == nil {
		t.Fatalf("sent message=%#v error=%v", message, err)
	}
	message, err = client.GetMessage(context.Background(), message.ID)
	if err != nil || message.ID != testChannelID+":1785571200.000003" || message.Text == nil || *message.Text != "history" || message.Direction != socialhub.DirectionOutbound {
		t.Fatalf("fetched message=%#v error=%v", message, err)
	}
	status, err := client.PublishStatus(context.Background(), post.ID)
	if err != nil || status.ID != post.ID || status.State != socialhub.PublishStatePublished {
		t.Fatalf("publish status=%#v error=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), post.ID); err != nil {
		t.Fatal(err)
	}
	if postCalls != 3 {
		t.Fatalf("post calls=%d", postCalls)
	}
}

func TestFetcherProfilesHistoryAndThreads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/api/")
		body := requireSlackRequest(t, writer, request, method)
		if body == nil {
			return
		}
		switch method {
		case "users.info":
			if body["user"] != testActorID {
				t.Errorf("users.info body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "user": map[string]any{
				"id": testActorID, "team_id": testWorkspaceID, "name": "ada", "real_name": "Ada Lovelace",
				"profile": map[string]any{"display_name": "Ada", "image_192": "https://cdn.test/ada.jpg"},
			}})
		case "conversations.history":
			if body["channel"] != testPrivateID || body["cursor"] != "cursor-1" || body["limit"] != float64(2) || body["oldest"] != "1785582000.000000" || body["latest"] != "1785585600.000000" {
				t.Errorf("history body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{
				"ok": true, "has_more": true, "response_metadata": map[string]any{"next_cursor": "cursor-2"},
				"messages": []any{
					map[string]any{"type": "message", "user": testActorID, "text": "one", "ts": testTimestamp},
					map[string]any{"type": "message", "text": "invalid"},
				},
			})
		case "conversations.replies":
			if body["channel"] != testPrivateID || body["ts"] != testTimestamp || body["cursor"] != "cursor-1" || body["limit"] != float64(15) {
				t.Errorf("replies body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{
				"ok": true, "has_more": false, "response_metadata": map[string]any{"next_cursor": "cursor-2"},
				"messages": []any{
					map[string]any{"type": "message", "user": testActorID, "text": "root", "ts": testTimestamp},
					map[string]any{"type": "message", "user": "U999ABC", "text": "reply", "ts": "1785571201.000002", "thread_ts": testTimestamp, "reactions": []any{map[string]any{"name": "eyes", "count": 1}}},
				},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, false, allTestScopes())
	user, err := client.GetUser(context.Background(), testActorID)
	if err != nil || user.ID != testActorID || user.Username == nil || *user.Username != "ada" || user.DisplayName == nil || *user.DisplayName != "Ada" || user.AvatarURL == nil || len(user.Extensions) != 1 {
		t.Fatalf("user=%#v error=%v", user, err)
	}
	start, end := testNow.Add(-time.Hour), testNow
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{
		UserID: testPrivateID, Cursor: "cursor-1", MaxResults: 2, StartTime: &start, EndTime: &end,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != testPrivateID+":"+testTimestamp || page.NextCursor == nil || *page.NextCursor != "cursor-2" || !page.HasMore || page.Items[0].Visibility == nil || *page.Items[0].Visibility != "private" {
		t.Fatalf("history page=%#v error=%v", page, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testPrivateID + ":" + testTimestamp, Cursor: "cursor-1"})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != testPrivateID+":1785571201.000002" || comments.Items[0].PostID != testPrivateID+":"+testTimestamp || comments.NextCursor == nil || *comments.NextCursor != "cursor-2" || !comments.HasMore {
		t.Fatalf("comments=%#v error=%v", comments, err)
	}
}

func TestChatAndFetchValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, false, allTestScopes())
	_, noDefault := newTestAdapter(t, server, "", false, allTestScopes())
	text, visibility, quote, reply := "text", "public", testChannelID+":"+testTimestamp, testPrivateID+":"+testTimestamp
	invalid := []func() error{
		func() error {
			_, err := noDefault.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, QuotePostID: &quote})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, Visibility: &visibility})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, MediaIDs: []string{testFileID}})
			return err
		},
		func() error {
			_, err := client.PostMessage(context.Background(), PostMessageRequest{ChannelID: "bad", Text: text})
			return err
		},
		func() error {
			_, err := client.PostMessage(context.Background(), PostMessageRequest{ChannelID: testChannelID, Text: " "})
			return err
		},
		func() error {
			_, err := client.PostMessage(context.Background(), PostMessageRequest{ChannelID: testChannelID, Text: text, ThreadPostID: reply})
			return err
		},
		func() error {
			_, err := client.PostMessage(context.Background(), PostMessageRequest{ChannelID: testChannelID, Text: text}, socialhub.WithIdempotencyKey("key"))
			return err
		},
		func() error {
			_, err := client.UpdateMessage(context.Background(), UpdateMessageRequest{PostID: "bad", Text: text})
			return err
		},
		func() error {
			_, err := client.UpdateMessage(context.Background(), UpdateMessageRequest{PostID: quote, Text: " "})
			return err
		},
		func() error { return client.DeletePost(context.Background(), "bad") },
		func() error {
			_, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: testChannelID, Text: &text, RecipientIDs: []string{testActorID}})
			return err
		},
		func() error {
			_, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: testChannelID, Text: &text, MediaIDs: []string{testFileID}})
			return err
		},
		func() error { _, err := client.GetMessage(context.Background(), "bad"); return err },
		func() error { _, err := client.GetUser(context.Background(), "bad"); return err },
		func() error { _, err := client.GetPost(context.Background(), "bad"); return err },
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: testChannelID, MaxResults: -1})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: testChannelID, Cursor: "bad\n"})
			return err
		},
		func() error {
			start, end := testNow, testNow.Add(-time.Second)
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: testChannelID, StartTime: &start, EndTime: &end})
			return err
		},
		func() error {
			_, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "bad"})
			return err
		},
	}
	for index, call := range invalid {
		if err := call(); err == nil {
			t.Fatalf("validation %d accepted", index)
		}
	}
	if _, ok := parseTimestamp(testTimestamp); !ok || validTimestamp("0") || validTimestamp("1.bad") || validTimestamp("1.1234567890") {
		t.Fatal("timestamp validation mismatch")
	}
	if channel, timestamp, err := parseCompositeID(testDMID+":"+testTimestamp, "test"); err != nil || channel != testDMID || timestamp != testTimestamp || conversationVisibility(testDMID) != "direct" {
		t.Fatalf("composite=%s/%s error=%v", channel, timestamp, err)
	}
	if limit, _ := slackPageLimit(2000); limit != 1000 {
		t.Fatalf("page limit=%d", limit)
	}
	if got := slackTime(testNow); got != "1785585600.000000" {
		t.Fatalf("Slack time=%s", got)
	}
	if !errors.Is(unsupported("test", "unsupported"), socialhub.ErrUnsupported) {
		t.Fatal("unsupported sentinel mismatch")
	}
}
