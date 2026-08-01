package lark

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

func wireMessageValue(id, text string) map[string]any {
	return map[string]any{
		"message_id": id, "root_id": "", "parent_id": "", "thread_id": "",
		"msg_type": "text", "create_time": "1785571200000", "update_time": "1785571201000",
		"deleted": false, "updated": false, "chat_id": testChatID,
		"sender":           map[string]any{"id": testActorID, "id_type": "app_id", "sender_type": "app", "tenant_key": testTenantKey},
		"body":             map[string]any{"content": `{"text":"` + text + `"}`},
		"message_app_link": "https://applink.test/message/" + id,
	}
}

func TestTypedMessageWorkflowContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/open-apis/im/v1/messages":
			body := requireLarkRequest(t, writer, request, TokenTenant)
			if body == nil {
				return
			}
			if request.URL.Query().Get("receive_id_type") != "open_id" || body["receive_id"] != testUserID || body["msg_type"] != "text" || body["content"] != `{"text":"hello"}` || body["uuid"] != "send-uuid" || request.Header.Get("X-Request-ID") != "request-1" {
				t.Errorf("send query=%v body=%v headers=%v", request.URL.Query(), body, request.Header)
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "msg": "success", "data": wireMessageValue(testMessageID, "hello")})
		case request.Method == http.MethodPost && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID+"/reply":
			body := requireLarkRequest(t, writer, request, TokenTenant)
			if body == nil {
				return
			}
			if body["msg_type"] != "post" || body["content"] != `{"zh_cn":{"title":"Update"}}` || body["reply_in_thread"] != true || body["uuid"] != "reply-uuid" {
				t.Errorf("reply body=%v", body)
			}
			value := wireMessageValue(testReplyID, "")
			value["msg_type"], value["root_id"], value["parent_id"], value["thread_id"] = "post", testMessageID, testMessageID, testThreadID
			value["body"] = map[string]any{"content": `{"zh_cn":{"title":"Update"}}`}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": value})
		case request.Method == http.MethodPut && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID:
			body := requireLarkRequest(t, writer, request, TokenTenant)
			if body == nil {
				return
			}
			if body["msg_type"] != "text" || body["content"] != `{"text":"edited"}` {
				t.Errorf("update body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": wireMessageValue(testMessageID, "edited")})
		case request.Method == http.MethodDelete && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID:
			if requireLarkRequest(t, writer, request, TokenTenant) == nil {
				return
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "msg": "success", "data": map[string]any{}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)

	sent, err := client.Send(context.Background(), SendRequest{
		ReceiveIDType: ReceiveOpenID, ReceiveID: testUserID, MessageType: "text", Content: json.RawMessage(`{"text":"hello"}`),
	}, socialhub.WithIdempotencyKey("send-uuid"), socialhub.WithRequestID("request-1"))
	if err != nil || sent.ID != testMessageID || sent.ConversationID != testChatID || sent.Direction != socialhub.DirectionOutbound || sent.Text == nil || *sent.Text != "hello" || sent.SentAt == nil {
		t.Fatalf("sent=%#v err=%v", sent, err)
	}
	reply, err := client.Reply(context.Background(), ReplyRequest{
		MessageID: testMessageID, MessageType: "post", Content: json.RawMessage(`{"zh_cn":{"title":"Update"}}`), ReplyInThread: true,
	}, socialhub.WithIdempotencyKey("reply-uuid"))
	if err != nil || reply.ID != testReplyID || reply.ReplyToID == nil || *reply.ReplyToID != testMessageID || reply.Text != nil {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	updated, err := client.Update(context.Background(), UpdateRequest{MessageID: testMessageID, MessageType: "text", Content: json.RawMessage(`{"text":"edited"}`)})
	if err != nil || updated.Text == nil || *updated.Text != "edited" {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	if err := client.Delete(context.Background(), testMessageID); err != nil {
		t.Fatal(err)
	}
}

func TestCommonMessageAndPublisherContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/open-apis/im/v1/messages":
			body := requireLarkRequest(t, writer, request, TokenTenant)
			if body == nil {
				return
			}
			if request.URL.Query().Get("receive_id_type") != "chat_id" || body["receive_id"] != testChatID {
				t.Errorf("common send query=%v body=%v", request.URL.Query(), body)
			}
			var content map[string]string
			_ = json.Unmarshal([]byte(body["content"].(string)), &content)
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": wireMessageValue(testMessageID, content["text"])})
		case request.Method == http.MethodPost && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID+"/reply":
			body := requireLarkRequest(t, writer, request, TokenTenant)
			if body == nil {
				return
			}
			value := wireMessageValue(testReplyID, "reply")
			value["root_id"], value["parent_id"] = testMessageID, testMessageID
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": value})
		case request.Method == http.MethodGet && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID:
			if requireLarkRequest(t, writer, request, TokenTenant) == nil {
				return
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{"items": []any{wireMessageValue(testMessageID, "hello")}}})
		case request.Method == http.MethodDelete && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID:
			if requireLarkRequest(t, writer, request, TokenTenant) == nil {
				return
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	text := "hello"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: testChatID, Text: &text})
	if err != nil || message.ID != testMessageID || message.Text == nil || *message.Text != text {
		t.Fatalf("message=%#v err=%v", message, err)
	}
	replyTo := testMessageID
	reply, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{Text: stringPointer("reply"), ReplyToID: &replyTo})
	if err != nil || reply.ID != testReplyID {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if err != nil || post.ID != testMessageID || post.Status == nil || post.URL == nil {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	status, err := client.PublishStatus(context.Background(), testMessageID)
	if err != nil || status.ID != testMessageID || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), testMessageID); err != nil {
		t.Fatal(err)
	}
}

func TestFetcherContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requireLarkRequest(t, writer, request, TokenTenant) == nil {
			return
		}
		switch {
		case request.URL.Path == "/open-apis/contact/v3/users/"+testUserID:
			if request.URL.Query().Get("user_id_type") != "open_id" || request.URL.Query().Get("department_id_type") != "open_department_id" {
				t.Errorf("user query=%v", request.URL.Query())
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{"user": map[string]any{
				"open_id": testUserID, "union_id": "on_union", "user_id": "employee-1", "name": "Test User", "en_name": "User",
				"avatar": map[string]any{"avatar_origin": "https://avatar.test/original"}, "status": map[string]any{"is_activated": true},
			}}})
		case request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID:
			root := wireMessageValue(testMessageID, "root")
			root["thread_id"] = testThreadID
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{"items": []any{root}}})
		case request.URL.Path == "/open-apis/im/v1/messages" && request.URL.Query().Get("container_id_type") == "thread":
			if request.URL.Query().Get("container_id") != testThreadID || request.URL.Query().Get("page_size") != "10" || request.URL.Query().Get("page_token") != "comment-cursor" {
				t.Errorf("thread query=%v", request.URL.Query())
			}
			reply := wireMessageValue(testReplyID, "reply")
			reply["root_id"], reply["parent_id"], reply["thread_id"] = testMessageID, testMessageID, testThreadID
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{
				"has_more": false, "page_token": "", "items": []any{wireMessageValue(testMessageID, "root"), reply},
			}})
		case request.URL.Path == "/open-apis/im/v1/messages":
			query := request.URL.Query()
			if query.Get("container_id_type") != "chat" || query.Get("container_id") != testChatID || query.Get("page_size") != "50" || query.Get("page_token") != "posts-cursor" || query.Get("sort_type") != "ByCreateTimeDesc" || query.Get("only_thread_root_messages") != "true" || query.Get("start_time") == "" || query.Get("end_time") == "" {
				t.Errorf("posts query=%v", query)
			}
			top := wireMessageValue(testMessageID, "root")
			reply := wireMessageValue(testReplyID, "reply")
			reply["root_id"] = testMessageID
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{
				"has_more": true, "page_token": "next-posts", "items": []any{top, reply, map[string]any{"message_id": "bad"}},
			}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)

	user, err := client.GetUser(context.Background(), testUserID)
	if err != nil || user.ID != testUserID || user.DisplayName == nil || *user.DisplayName != "Test User" || user.AvatarURL == nil {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	message, err := client.GetMessage(context.Background(), testMessageID)
	if err != nil || message.ID != testMessageID || message.Text == nil || *message.Text != "root" {
		t.Fatalf("message=%#v err=%v", message, err)
	}
	post, err := client.GetPost(context.Background(), testMessageID)
	if err != nil || post.ID != testMessageID || post.CreatedAt == nil {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	start, end := testNow.Add(-time.Hour), testNow
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{
		UserID: testChatID, Cursor: "posts-cursor", MaxResults: 100, StartTime: &start, EndTime: &end,
	})
	if err != nil || len(posts.Items) != 1 || posts.NextCursor == nil || *posts.NextCursor != "next-posts" || !posts.HasMore {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: testMessageID, Cursor: "comment-cursor", MaxResults: 10})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != testReplyID || comments.Items[0].PostID != testMessageID {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
}

func TestMessageAndFetchValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	badSends := []SendRequest{
		{},
		{ReceiveIDType: "bad", ReceiveID: testUserID, MessageType: "text", Content: json.RawMessage(`{"text":"x"}`)},
		{ReceiveIDType: ReceiveOpenID, ReceiveID: "bad/id", MessageType: "text", Content: json.RawMessage(`{"text":"x"}`)},
		{ReceiveIDType: ReceiveOpenID, ReceiveID: testUserID, MessageType: "bad", Content: json.RawMessage(`{}`)},
		{ReceiveIDType: ReceiveOpenID, ReceiveID: testUserID, MessageType: "text", Content: json.RawMessage(`[]`)},
	}
	for index, input := range badSends {
		if _, err := client.Send(context.Background(), input); err == nil {
			t.Fatalf("bad send %d accepted", index)
		}
	}
	validSend := SendRequest{ReceiveIDType: ReceiveOpenID, ReceiveID: testUserID, MessageType: "text", Content: json.RawMessage(`{"text":"x"}`)}
	if _, err := client.Send(context.Background(), validSend, socialhub.WithIdempotencyKey(strings.Repeat("x", 51))); err == nil {
		t.Fatal("long UUID accepted")
	}
	if _, err := client.Update(context.Background(), UpdateRequest{MessageID: "bad", MessageType: "image", Content: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("bad update accepted")
	}
	if err := client.Delete(context.Background(), "bad"); err == nil {
		t.Fatal("bad delete accepted")
	}
	text := "x"
	badCommon := []socialhub.SendMessageRequest{
		{},
		{ConversationID: "bad", Text: &text},
		{ConversationID: testChatID, RecipientIDs: []string{testUserID}, Text: &text},
		{RecipientIDs: []string{testUserID, "ou_other"}, Text: &text},
		{ConversationID: testChatID, Text: &text, MediaIDs: []string{testResourceKey}},
	}
	for index, input := range badCommon {
		if _, err := client.SendMessage(context.Background(), input); err == nil {
			t.Fatalf("bad common send %d accepted", index)
		}
	}
	quote := testMessageID
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, QuotePostID: &quote}); err == nil {
		t.Fatal("quote publish accepted")
	}
	if _, err := client.GetUser(context.Background(), "bad/id"); err == nil {
		t.Fatal("bad user accepted")
	}
	if _, err := client.GetMessage(context.Background(), "bad"); err == nil {
		t.Fatal("bad message accepted")
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: "bad"}); err == nil {
		t.Fatal("bad chat accepted")
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: testChatID, Cursor: "bad\n"}); err == nil {
		t.Fatal("bad cursor accepted")
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: testChatID, MaxResults: -1}); err == nil {
		t.Fatal("negative max accepted")
	}
	start, end := testNow, testNow.Add(-time.Hour)
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{UserID: testChatID, StartTime: &start, EndTime: &end}); err == nil {
		t.Fatal("reversed time accepted")
	}
	if _, err := client.GetMessage(context.Background(), testMessageID, socialhub.WithFields("body")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("field selection error=%v", err)
	}
	_, userClient := newTestClient(t, server, TokenUser, testChatID, testActorID, false)
	if _, err := userClient.Update(context.Background(), UpdateRequest{MessageID: testMessageID, MessageType: "text", Content: json.RawMessage(`{"text":"x"}`)}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("user update error=%v", err)
	}
}

func TestMappingHelpers(t *testing.T) {
	if selectedUserID(wireUser{OpenID: "open", UnionID: "union", UserID: "user"}, UserIDOpenID) != "open" || selectedUserID(wireUser{OpenID: "open", UnionID: "union", UserID: "user"}, UserIDUnionID) != "union" || selectedUserID(wireUser{OpenID: "open", UnionID: "union", UserID: "user"}, UserIDUserID) != "user" {
		t.Fatal("user ID selection mismatch")
	}
	if parsed, ok := larkTime("1785571200"); !ok || parsed.Unix() != 1785571200 {
		t.Fatalf("seconds parsed=%v ok=%v", parsed, ok)
	}
	if _, ok := larkTime("bad"); ok {
		t.Fatal("bad timestamp accepted")
	}
	if firstUserID(eventUserID{UnionID: "union"}) != "union" || firstNonEmpty("", " x ") != "x" || stringPointer("") != nil {
		t.Fatal("mapping helper mismatch")
	}
}
