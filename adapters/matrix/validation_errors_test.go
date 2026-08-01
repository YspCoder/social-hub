package matrix

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestValidationAndIdentifierHelpers(t *testing.T) {
	for name, valid := range map[string]bool{
		"user":       validUserID("@alice:example.test"),
		"room":       validRoomID("!room:example.test"),
		"event":      validEventID("$event/one:example.test"),
		"opaque":     validOpaque("token/one", 32),
		"text":       validText("hello\nworld"),
		"filename":   validFilename("photo name.png"),
		"mime":       validMIME("text/plain; charset=utf-8"),
		"mxc":        validMXCURI("mxc://example.test/media-id"),
		"homeserver": validHomeserverURL("https://matrix.example.test/"),
	} {
		if !valid {
			t.Fatalf("%s should be valid", name)
		}
	}
	for name, valid := range map[string]bool{
		"user":              validUserID("alice"),
		"room":              validRoomID("!:server"),
		"event":             validEventID("event"),
		"opaque whitespace": validOpaque(" token", 32),
		"opaque control":    validOpaque("bad\nvalue", 32),
		"text":              validText("\x00"),
		"filename path":     validFilename("dir/photo.png"),
		"filename dot":      validFilename(".."),
		"mime":              validMIME("image/*"),
		"mxc query":         validMXCURI("mxc://example.test/id?q=1"),
		"mxc path":          validMXCURI("mxc://example.test/a/b"),
		"homeserver path":   validHomeserverURL("https://matrix.example.test/client"),
		"homeserver scheme": validHomeserverURL("ftp://matrix.example.test"),
	} {
		if valid {
			t.Fatalf("%s should be invalid", name)
		}
	}
	transaction, err := randomTransactionID()
	if err != nil || !strings.HasPrefix(transaction, "socialhub_") {
		t.Fatalf("transaction=%q err=%v", transaction, err)
	}
	if value, err := transactionID(socialhub.WithIdempotencyKey("stable/one")); err != nil || value != "stable/one" {
		t.Fatalf("transaction=%q err=%v", value, err)
	}
	if _, err := transactionID(socialhub.WithIdempotencyKey("bad\nvalue")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid transaction error=%v", err)
	}
	if _, err := transactionID(socialhub.WithCallTimeout(-time.Second)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("call option error=%v", err)
	}

	composite := composeID("!room:example.test", "$event:example.test")
	roomID, eventID, err := parseCompositeID("test", composite, "")
	if err != nil || roomID != "!room:example.test" || eventID != "$event:example.test" {
		t.Fatalf("parsed=%q %q err=%v", roomID, eventID, err)
	}
	roomID, eventID, err = parseCompositeID("test", "$event:example.test", "!room:example.test")
	if err != nil || roomID != "!room:example.test" || eventID != "$event:example.test" {
		t.Fatalf("raw parsed=%q %q err=%v", roomID, eventID, err)
	}
	for _, value := range []string{"bad", "mx:bad", "mx:!.!"} {
		if _, _, err := parseCompositeID("test", value, ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("value=%q error=%v", value, err)
		}
	}
	if matrixTime(0) != nil || matrixTime(testNow.UnixMilli()) == nil || matrixToURL("!room:example.test", "$event:example.test") == "" {
		t.Fatal("time or permalink mapping failed")
	}
}

func TestWorkflowValidationBoundaries(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	ctx := context.Background()
	invalidCases := []func() error{
		func() error { _, err := client.GetUser(ctx, "alice"); return err },
		func() error { _, err := client.GetEvent(ctx, "room", "event"); return err },
		func() error { _, err := client.ListRoomMessages(ctx, RoomMessagesRequest{RoomID: "bad"}); return err },
		func() error {
			_, err := client.ListRoomMessages(ctx, RoomMessagesRequest{RoomID: "!room:example.test", Direction: "x"})
			return err
		},
		func() error {
			_, err := client.ListRoomMessages(ctx, RoomMessagesRequest{RoomID: "!room:example.test", MaxResults: -1})
			return err
		},
		func() error { _, err := client.SendText(ctx, SendTextRequest{RoomID: "bad", Text: "x"}); return err },
		func() error {
			_, err := client.SendText(ctx, SendTextRequest{RoomID: "!room:example.test", MessageType: MessageTypeImage, Text: "x"})
			return err
		},
		func() error {
			_, err := client.SendText(ctx, SendTextRequest{RoomID: "!room:example.test", Text: "x", ReplyToID: "bad"})
			return err
		},
		func() error {
			_, err := client.SendMedia(ctx, SendMediaRequest{RoomID: "!room:example.test", MessageType: MessageTypeImage, Body: "x", MXCURI: "https://example.test/x"})
			return err
		},
		func() error {
			_, err := client.SendMedia(ctx, SendMediaRequest{RoomID: "!room:example.test", MessageType: MessageTypeText, Body: "x", MXCURI: "mxc://example.test/x"})
			return err
		},
		func() error {
			_, err := client.SendMedia(ctx, SendMediaRequest{RoomID: "!room:example.test", MessageType: MessageTypeFile, Body: "x", MXCURI: "mxc://example.test/x", MIME: "bad"})
			return err
		},
		func() error {
			_, err := client.SendReaction(ctx, ReactionEventRequest{RoomID: "!room:example.test", TargetEventID: "bad", Key: "x"})
			return err
		},
		func() error { _, err := client.Redact(ctx, "bad", "$event:example.test", ""); return err },
		func() error { _, err := client.GetPost(ctx, "bad"); return err },
		func() error { _, err := client.Publish(ctx, socialhub.CreatePostRequest{}); return err },
		func() error { _, err := client.ListPosts(ctx, socialhub.ListPostsRequest{MaxResults: -1}); return err },
		func() error { _, err := client.SendMessage(ctx, socialhub.SendMessageRequest{}); return err },
		func() error { _, err := client.GetMessage(ctx, "bad"); return err },
		func() error {
			return client.React(ctx, socialhub.ReactionRequest{TargetID: "bad", Kind: socialhub.ReactionRepost})
		},
		func() error {
			_, err := client.ListComments(ctx, socialhub.ListCommentsRequest{PostID: "bad"})
			return err
		},
		func() error {
			_, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: "bad", Text: "x"})
			return err
		},
		func() error { return client.DeletePost(ctx, "bad") },
		func() error { return client.DeleteComment(ctx, "bad") },
	}
	for index, call := range invalidCases {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d error=%v", index, err)
		}
	}

	text := "hello"
	visibility := "public"
	if _, err := client.Publish(ctx, socialhub.CreatePostRequest{Text: &text, Visibility: &visibility}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("publish feature error=%v", err)
	}
	if _, err := client.ListPosts(ctx, socialhub.ListPostsRequest{UserID: "@alice:example.test"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("user filter error=%v", err)
	}
	if _, err := client.ListPosts(ctx, socialhub.ListPostsRequest{StartTime: &testNow}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("time filter error=%v", err)
	}
	if _, err := client.SendMessage(ctx, socialhub.SendMessageRequest{ConversationID: "!room:example.test", Text: &text, RecipientIDs: []string{"@alice:example.test"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("recipient error=%v", err)
	}
	crossRoom := composeID("!other:example.test", "$event:example.test")
	if _, err := client.Publish(ctx, socialhub.CreatePostRequest{Text: &text, ReplyToID: &crossRoom}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cross-room publish error=%v", err)
	}
	if _, err := client.SendMessage(ctx, socialhub.SendMessageRequest{ConversationID: "!room:example.test", Text: &text, ReplyToID: &crossRoom}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cross-room message error=%v", err)
	}
	badParent := composeID("!other:example.test", "$comment:example.test")
	if _, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: composeID("!room:example.test", "$root:example.test"), ParentID: &badParent, Text: "reply"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("cross-room comment error=%v", err)
	}
	if _, err := client.ListComments(ctx, socialhub.ListCommentsRequest{PostID: composeID("!room:example.test", "$root:example.test"), Cursor: "bad\nvalue"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment cursor error=%v", err)
	}
}

func TestMappingRejectsUnsupportedAndMalformedEvents(t *testing.T) {
	events := []Event{
		{Type: EventTypeEncrypted, EventID: "$one:example.test", Content: []byte(`{}`)},
		{Type: EventTypeReaction, EventID: "$one:example.test", Content: []byte(`{}`)},
	}
	for _, event := range events {
		if _, err := messageContent(event); !errors.Is(err, socialhub.ErrUnsupported) {
			t.Fatalf("event=%#v error=%v", event, err)
		}
	}
	for _, content := range []string{`[`, `{"msgtype":"custom","body":"x"}`} {
		if _, err := messageContent(Event{Type: EventTypeMessage, EventID: "$one:example.test", Content: []byte(content)}); errorCode(err) != socialhub.CodePlatformError {
			t.Fatalf("content=%q error=%v", content, err)
		}
	}
	content, err := messageContent(Event{Type: EventTypeMessage, EventID: "$one:example.test", Content: []byte(`{"msgtype":"m.text","body":""}`)})
	if err != nil || content.Body != "" || len(messageMedia("$one:example.test", content)) != 0 || replyEventID(nil) != "" {
		t.Fatalf("content=%#v err=%v", content, err)
	}
}

func TestRequestAndHTTPErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	if _, err := client.newRequest(context.Background(), http.MethodGet, "/wrong", nil, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("path error=%v", err)
	}
	if _, err := client.newRequest(context.Background(), http.MethodGet, "/_matrix/%zz", nil, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("encoding error=%v", err)
	}
	if err := client.json(context.Background(), http.MethodPost, "/_matrix/client/v3/test", nil, make(chan int), nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("marshal error=%v", err)
	}

	header := make(http.Header)
	header.Set("Retry-After", "2.5")
	header.Set("X-Request-ID", "request-1")
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"errcode":"M_LIMIT_EXCEEDED","error":"slow down","retry_after_ms":9000}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrRateLimited) || platformErr.RetryAfter != 2500*time.Millisecond || platformErr.RequestID != "request-1" || !platformErr.Retryable() {
		t.Fatalf("error=%#v", err)
	}
	err = decodeHTTPError(http.StatusTooManyRequests, nil, []byte(`{"errcode":"M_LIMIT_EXCEEDED","retry_after_ms":1200}`))
	if !errors.As(err, &platformErr) || platformErr.RetryAfter != 1200*time.Millisecond {
		t.Fatalf("json retry error=%#v", err)
	}
	err = decodeHTTPError(http.StatusUnauthorized, nil, []byte(`{"errcode":"M_UNKNOWN_TOKEN","error":"expired","soft_logout":true}`))
	if !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("auth error=%v", err)
	}

	tests := []struct {
		status int
		code   string
		want   socialhub.ErrorCode
	}{
		{http.StatusBadRequest, "M_BAD_JSON", socialhub.CodeInvalidArgument},
		{http.StatusUnauthorized, "M_UNAUTHORIZED", socialhub.CodeUnauthenticated},
		{http.StatusForbidden, "M_FORBIDDEN", socialhub.CodePermissionDenied},
		{http.StatusNotFound, "M_NOT_FOUND", socialhub.CodeNotFound},
		{http.StatusConflict, "M_CANNOT_OVERWRITE_MEDIA", socialhub.CodeConflict},
		{http.StatusTooManyRequests, "M_RESOURCE_LIMIT_EXCEEDED", socialhub.CodeRateLimited},
		{http.StatusGatewayTimeout, "M_NOT_YET_UPLOADED", socialhub.CodeTemporarilyUnavailable},
		{http.StatusBadRequest, "M_UNRECOGNIZED", socialhub.CodeUnsupported},
		{http.StatusUnprocessableEntity, "", socialhub.CodeInvalidArgument},
		{http.StatusUnauthorized, "", socialhub.CodeUnauthenticated},
		{http.StatusForbidden, "", socialhub.CodePermissionDenied},
		{http.StatusGone, "", socialhub.CodeNotFound},
		{http.StatusConflict, "", socialhub.CodeConflict},
		{http.StatusTooManyRequests, "", socialhub.CodeRateLimited},
		{http.StatusServiceUnavailable, "", socialhub.CodeTemporarilyUnavailable},
		{http.StatusTeapot, "", socialhub.CodePlatformError},
	}
	for _, test := range tests {
		actual, _ := classifyError(test.status, test.code)
		if actual != test.want {
			t.Fatalf("status=%d code=%q actual=%q want=%q", test.status, test.code, actual, test.want)
		}
	}
	if retryAfterHeader("bad") != 0 || retryAfterHeader("90000") != 0 || boundedMessage(strings.Repeat("界", 520), 512) != strings.Repeat("界", 512) || firstNonEmpty("", "value") != "value" {
		t.Fatal("error helper behavior is invalid")
	}
}

func errorCode(err error) socialhub.ErrorCode {
	var platformErr *socialhub.Error
	if errors.As(err, &platformErr) {
		return platformErr.Code
	}
	return ""
}
