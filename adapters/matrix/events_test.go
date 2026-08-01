package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

type capturedEvent struct {
	path        string
	eventType   string
	content     MessageContent
	relation    Relation
	redaction   bool
	requestID   string
	idempotency string
}

func TestEventAndCommonCapabilityContracts(t *testing.T) {
	const (
		roomID     = "!room/alpha:example.test"
		mediaEvent = "$media/event:example.test"
		rootEvent  = "$root:example.test"
	)
	var captured []capturedEvent
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer matrix-token" || request.URL.Query().Get("access_token") != "" {
			writeMatrixJSON(writer, http.StatusUnauthorized, `{"errcode":"M_UNKNOWN_TOKEN"}`)
			return
		}
		escapedPath := request.URL.EscapedPath()
		switch {
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/_matrix/client/v3/profile/"):
			if !strings.Contains(escapedPath, "%2F") {
				writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_INVALID_PARAM"}`)
				return
			}
			writeMatrixJSON(writer, http.StatusOK, `{"displayname":"Alice","avatar_url":"mxc://example.test/avatar"}`)
		case request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/event/"):
			if strings.Contains(request.URL.Path, "$published:example.test") {
				writeMatrixJSON(writer, http.StatusOK, `{"type":"m.room.message","event_id":"$published:example.test","sender":"@hub:example.test","origin_server_ts":1785657600000,"content":{"msgtype":"m.text","body":"published"}}`)
				return
			}
			if !strings.Contains(escapedPath, "%2F") {
				writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_INVALID_PARAM"}`)
				return
			}
			writeMatrixJSON(writer, http.StatusOK, `{"type":"m.room.message","event_id":"$media/event:example.test","sender":"@alice:example.test","origin_server_ts":1785657600000,"content":{"msgtype":"m.image","body":"photo.png","url":"mxc://example.test/photo","info":{"mimetype":"image/png","size":12,"w":10,"h":20},"m.relates_to":{"m.in_reply_to":{"event_id":"$root:example.test"}}}}`)
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/messages"):
			if request.URL.Query().Get("from") != "cursor" || request.URL.Query().Get("limit") != "2" {
				writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_INVALID_PARAM"}`)
				return
			}
			writeMatrixJSON(writer, http.StatusOK, `{"start":"cursor","end":"next","chunk":[{"type":"m.room.message","event_id":"$timeline:example.test","sender":"@alice:example.test","origin_server_ts":1785657600000,"content":{"msgtype":"m.text","body":"timeline"}},{"type":"m.reaction","event_id":"$reaction-in:example.test","sender":"@alice:example.test","origin_server_ts":1785657600000,"content":{"m.relates_to":{"rel_type":"m.annotation","event_id":"$timeline:example.test","key":"ok"}}}]}`)
		case request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/relations/"):
			if request.URL.Query().Get("from") != "comments" || request.URL.Query().Get("limit") != "2" {
				writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_INVALID_PARAM"}`)
				return
			}
			writeMatrixJSON(writer, http.StatusOK, `{"chunk":[{"type":"m.room.message","event_id":"$comment-remote:example.test","sender":"@alice:example.test","origin_server_ts":1785657600000,"content":{"msgtype":"m.text","body":"remote reply","m.relates_to":{"rel_type":"m.thread","event_id":"$root:example.test","m.in_reply_to":{"event_id":"$root:example.test"}}}}],"next_batch":"comments-next"}`)
		case request.Method == http.MethodPut && strings.Contains(request.URL.Path, "/send/"):
			entry := capturedEvent{path: escapedPath, requestID: request.Header.Get("X-Request-ID"), idempotency: request.Header.Get("Idempotency-Key")}
			if strings.Contains(request.URL.Path, "/send/m.reaction/") {
				entry.eventType = EventTypeReaction
				var input struct {
					Relation Relation `json:"m.relates_to"`
				}
				if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
					writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_BAD_JSON"}`)
					return
				}
				entry.relation = input.Relation
				captured = append(captured, entry)
				writeMatrixJSON(writer, http.StatusOK, `{"event_id":"$reaction-sent:example.test"}`)
				return
			}
			entry.eventType = EventTypeMessage
			if err := json.NewDecoder(request.Body).Decode(&entry.content); err != nil {
				writeMatrixJSON(writer, http.StatusBadRequest, `{"errcode":"M_BAD_JSON"}`)
				return
			}
			captured = append(captured, entry)
			eventID := "$published:example.test"
			switch {
			case entry.content.MessageType == MessageTypeImage:
				eventID = "$media-sent:example.test"
			case entry.content.RelatesTo != nil && entry.content.RelatesTo.RelationType == RelationThread:
				eventID = "$comment-sent:example.test"
			case entry.content.Body == "outbound":
				eventID = "$outbound:example.test"
			}
			writeMatrixJSON(writer, http.StatusOK, `{"event_id":"`+eventID+`"}`)
		case request.Method == http.MethodPut && strings.Contains(request.URL.Path, "/redact/"):
			captured = append(captured, capturedEvent{path: escapedPath, redaction: true})
			writeMatrixJSON(writer, http.StatusOK, `{"event_id":"$redacted:example.test"}`)
		default:
			writeMatrixJSON(writer, http.StatusNotFound, `{"errcode":"M_NOT_FOUND"}`)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, true)

	user, err := client.GetUser(context.Background(), "@alice/slash:example.test")
	if err != nil || user.Username == nil || *user.Username != "alice/slash" || user.DisplayName == nil || *user.DisplayName != "Alice" || user.AvatarURL == nil {
		t.Fatalf("user=%#v err=%v", user, err)
	}
	event, err := client.GetEvent(context.Background(), roomID, mediaEvent)
	if err != nil || event.RoomID != roomID || event.EventID != mediaEvent {
		t.Fatalf("event=%#v err=%v", event, err)
	}
	postID := composeID(roomID, mediaEvent)
	post, err := client.GetPost(context.Background(), postID)
	if err != nil || post.ID != postID || post.Text == nil || *post.Text != "photo.png" || len(post.Media) != 1 || post.Media[0].Type != socialhub.MediaTypeImage || len(post.Relations) != 1 {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	message, err := client.GetMessage(context.Background(), postID)
	if err != nil || message.Direction != socialhub.DirectionInbound || message.ReplyToID == nil || *message.ReplyToID != composeID(roomID, rootEvent) || len(message.Media) != 1 {
		t.Fatalf("message=%#v err=%v", message, err)
	}

	page, err := client.ListRoomMessages(context.Background(), RoomMessagesRequest{RoomID: roomID, Cursor: "cursor", MaxResults: 2, Direction: "f"})
	if err != nil || len(page.Items) != 2 || page.Items[0].RoomID != roomID || page.NextCursor == nil || *page.NextCursor != "next" {
		t.Fatalf("room page=%#v err=%v", page, err)
	}
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{Cursor: "cursor", MaxResults: 2})
	if err != nil || len(posts.Items) != 1 || posts.Items[0].Text == nil || *posts.Items[0].Text != "timeline" || !posts.HasMore {
		t.Fatalf("posts=%#v err=%v", posts, err)
	}

	thread, err := client.SendText(context.Background(), SendTextRequest{RoomID: roomID, Text: "threaded", ReplyToID: "$parent:example.test", ThreadRootID: rootEvent}, socialhub.WithIdempotencyKey("txn/one"), socialhub.WithRequestID("request-1"))
	if err != nil || thread.EventID != "$comment-sent:example.test" {
		t.Fatalf("thread=%#v err=%v", thread, err)
	}
	media, err := client.SendMedia(context.Background(), SendMediaRequest{RoomID: roomID, MessageType: MessageTypeImage, Body: "image.png", MXCURI: "mxc://example.test/image", MIME: "image/png", Size: 12, Width: 10, Height: 20})
	if err != nil || media.EventID != "$media-sent:example.test" {
		t.Fatalf("media=%#v err=%v", media, err)
	}

	text := "published"
	replyTo := composeID(roomID, rootEvent)
	published, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &replyTo}, socialhub.WithIdempotencyKey("publish-1"))
	if err != nil || published.ID != composeID(roomID, "$published:example.test") || published.CreatedAt == nil || !published.CreatedAt.Equal(testNow) {
		t.Fatalf("published=%#v err=%v", published, err)
	}
	status, err := client.PublishStatus(context.Background(), published.ID)
	if err != nil || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), published.ID, socialhub.WithIdempotencyKey("redact-post")); err != nil {
		t.Fatal(err)
	}

	outbound := "outbound"
	sent, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{Text: &outbound, ReplyToID: &replyTo}, socialhub.WithIdempotencyKey("message-1"))
	if err != nil || sent.ID != composeID(roomID, "$outbound:example.test") || sent.Direction != socialhub.DirectionOutbound || sent.SentAt == nil || !sent.SentAt.Equal(testNow) {
		t.Fatalf("sent=%#v err=%v", sent, err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "@hub:example.test", TargetID: postID, Kind: socialhub.ReactionLike}, socialhub.WithIdempotencyKey("reaction-1")); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("remove reaction error=%v", err)
	}

	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: composeID(roomID, rootEvent), Cursor: "comments", MaxResults: 2})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].PostID != composeID(roomID, rootEvent) || comments.Items[0].ParentID != nil || comments.NextCursor == nil {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
	parent := composeID(roomID, "$parent:example.test")
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: composeID(roomID, rootEvent), ParentID: &parent, Text: "local reply"}, socialhub.WithIdempotencyKey("comment-1"))
	if err != nil || comment.ID != composeID(roomID, "$comment-sent:example.test") || comment.ParentID == nil || *comment.ParentID != parent {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID, socialhub.WithIdempotencyKey("redact-comment")); err != nil {
		t.Fatal(err)
	}

	var sawEscapedTransaction, sawThread, sawMedia, sawReaction bool
	redactions := 0
	for _, entry := range captured {
		if strings.Contains(entry.path, "%2F") && entry.idempotency == "txn/one" && entry.requestID == "request-1" {
			sawEscapedTransaction = true
		}
		if entry.content.RelatesTo != nil && entry.content.RelatesTo.RelationType == RelationThread && entry.content.RelatesTo.EventID == rootEvent {
			sawThread = true
		}
		if entry.content.MessageType == MessageTypeImage && entry.content.URL == "mxc://example.test/image" && entry.content.Info != nil && entry.content.Info.Size == 12 {
			sawMedia = true
		}
		if entry.eventType == EventTypeReaction && entry.relation.RelationType == RelationAnnotation && entry.relation.Key == "👍" {
			sawReaction = true
		}
		if entry.redaction {
			redactions++
		}
	}
	if !sawEscapedTransaction || !sawThread || !sawMedia || !sawReaction || redactions != 2 {
		t.Fatalf("captured=%#v", captured)
	}
}

func writeMatrixJSON(writer http.ResponseWriter, status int, value string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(value))
}
