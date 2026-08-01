package qq

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTypedMessageWireContracts(t *testing.T) {
	type exchange struct {
		path     string
		payload  map[string]any
		response map[string]any
	}
	exchanges := []exchange{
		{
			path: "/v2/users/user-openid/messages",
			payload: map[string]any{
				"msg_type": float64(0), "content": "hello", "msg_id": "reply-1", "msg_seq": float64(2),
				"message_reference": map[string]any{"message_id": "reference-1"},
			},
			response: map[string]any{"id": "message-1", "timestamp": "2026-08-01T12:00:01Z", "ext_info": map[string]any{"ref_idx": "ref-1"}},
		},
		{
			path:     "/v2/groups/group-openid/messages",
			payload:  map[string]any{"msg_type": float64(2), "markdown": map[string]any{"content": "**hello**"}, "event_id": "event-1"},
			response: map[string]any{"id": "message-2"},
		},
		{
			path:     "/v2/groups/group-openid/messages",
			payload:  map[string]any{"msg_type": float64(7), "media": map[string]any{"file_info": "file-info"}, "is_wakeup": true},
			response: map[string]any{"id": "message-3"},
		},
		{
			path:     "/channels/channel-id/messages",
			payload:  map[string]any{"image": "https://cdn.example.test/image.png"},
			response: map[string]any{"id": "message-4"},
		},
	}
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if index >= len(exchanges) {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		want := exchanges[index]
		index++
		if request.Method != http.MethodPost || request.URL.Path != want.path || request.Header.Get("Authorization") != "QQBot access-token" {
			t.Errorf("request=%s %s auth=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || !reflect.DeepEqual(payload, want.payload) {
			t.Errorf("payload=%#v want=%#v err=%v", payload, want.payload, err)
		}
		writeTestJSON(t, writer, want.response)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)

	first, err := client.Send(context.Background(), MessageRequest{
		Target: Target{Scene: SceneC2C, ID: "user-openid"}, Content: TextContent{Text: "hello"},
		ReplyToID: "reply-1", Sequence: 2, ReferenceID: "reference-1",
	})
	if err != nil || first.ID != "message-1" || first.SentAt == nil || !first.SentAt.Equal(testNow.Add(time.Second)) || first.ReferenceIndex != "ref-1" {
		t.Fatalf("text result=%#v err=%v", first, err)
	}
	if _, err := client.Send(context.Background(), MessageRequest{
		Target: Target{Scene: SceneGroup, ID: "group-openid"}, Content: MarkdownContent{Markdown: "**hello**"}, EventID: "event-1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Send(context.Background(), MessageRequest{
		Target: Target{Scene: SceneGroup, ID: "group-openid"}, Content: MediaContent{FileInfo: "file-info"}, Wakeup: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Send(context.Background(), MessageRequest{
		Target: Target{Scene: SceneChannel, ID: "channel-id"}, Content: ChannelImageContent{URL: "https://cdn.example.test/image.png"},
	}); err != nil {
		t.Fatal(err)
	}
	if index != len(exchanges) {
		t.Fatalf("requests=%d want=%d", index, len(exchanges))
	}
}

func TestCommonMessageAuditAndRetract(t *testing.T) {
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call++
		switch call {
		case 1:
			if request.Method != http.MethodPost || request.URL.Path != "/v2/groups/group-id/messages" {
				t.Errorf("send request=%s %s", request.Method, request.URL.Path)
			}
			writeTestJSON(t, writer, map[string]any{"code": 304023, "message": "pending", "data": map[string]any{"message_audit": map[string]any{"audit_id": "audit-1"}}})
		case 2:
			if request.Method != http.MethodDelete || request.URL.Path != "/v2/groups/group-id/messages/message%2Fid" || request.URL.EscapedPath() != "/v2/groups/group-id/messages/message%252Fid" {
				t.Errorf("retract request=%s path=%s escaped=%s", request.Method, request.URL.Path, request.URL.EscapedPath())
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected call %d", call)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	text := "hello"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "group:group-id", Text: &text})
	if err != nil || message.ID != "audit:audit-1" || message.ConversationID != "group:group-id" || message.Direction != socialhub.DirectionOutbound || len(message.Extensions["qq.delivery"]) == 0 {
		t.Fatalf("message=%#v err=%v", message, err)
	}
	if err := client.Retract(context.Background(), Target{Scene: SceneGroup, ID: "group-id"}, "message%2Fid"); err != nil {
		t.Fatal(err)
	}
}

func TestMessageBusinessErrorsAndTokenInvalidation(t *testing.T) {
	responses := []map[string]any{
		{"code": 304024, "data": map[string]any{"message_audit": map[string]any{"audit_id": "audit-2"}}},
		{"code": 304023, "data": map[string]any{"message_audit": map[string]any{}}},
		{"code": 100007, "message": "token expired"},
		{"id": "message-1", "timestamp": "not-time"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		response := responses[0]
		responses = responses[1:]
		writeTestJSON(t, writer, response)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, false)
	spy := &invalidationSpy{}
	client.invalidator = spy
	request := MessageRequest{Target: Target{Scene: SceneC2C, ID: "user"}, Content: TextContent{Text: "hello"}}
	result, err := client.Send(context.Background(), request)
	if err != nil || !result.PendingAudit || result.AuditID != "audit-2" {
		t.Fatalf("audit result=%#v err=%v", result, err)
	}
	if _, err := client.Send(context.Background(), request); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("missing audit ID=%v", err)
	}
	if _, err := client.Send(context.Background(), request); !errors.Is(err, socialhub.ErrUnauthenticated) || spy.calls != 1 {
		t.Fatalf("token error=%v invalidations=%d", err, spy.calls)
	}
	if _, err := client.Send(context.Background(), request); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("bad timestamp=%v", err)
	}
}

func TestMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false)
	valid := Target{Scene: SceneC2C, ID: "user"}
	requests := []MessageRequest{
		{Target: Target{Scene: "bad", ID: "user"}, Content: TextContent{Text: "x"}},
		{Target: valid},
		{Target: valid, Content: TextContent{Text: "x"}, ReplyToID: "reply", EventID: "event"},
		{Target: valid, Content: TextContent{Text: "x"}, Sequence: -1},
		{Target: Target{Scene: SceneChannel, ID: "channel"}, Content: TextContent{Text: "x"}, Sequence: 1},
		{Target: valid, Content: TextContent{Text: "x"}, ReferenceID: "bad/id"},
		{Target: valid, Content: TextContent{}},
		{Target: valid, Content: MarkdownContent{}},
		{Target: Target{Scene: SceneChannel, ID: "channel"}, Content: MediaContent{FileInfo: "file"}},
		{Target: valid, Content: ChannelImageContent{URL: "https://example.test/image.png"}},
	}
	for index, request := range requests {
		if _, err := client.Send(context.Background(), request); err == nil {
			t.Fatalf("request %d unexpectedly succeeded", index)
		}
	}
	if _, err := ParseConversationID("missing-prefix"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("conversation ID=%v", err)
	}
	for _, input := range []socialhub.SendMessageRequest{
		{ConversationID: "c2c:user", RecipientIDs: []string{"other"}, Text: ptr("x")},
		{ConversationID: "c2c:user", MediaIDs: []string{"media"}, Text: ptr("x")},
		{ConversationID: "c2c:user"},
	} {
		if _, err := client.SendMessage(context.Background(), input); err == nil {
			t.Fatalf("common request %#v unexpectedly succeeded", input)
		}
	}
	if _, err := client.GetMessage(context.Background(), "message"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message=%v", err)
	}
}

type invalidationSpy struct{ calls int }

func (spy *invalidationSpy) Invalidate(context.Context) { spy.calls++ }

func ptr(value string) *string { return &value }
