package vk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestReactionsCommentsAndMessagesContracts(t *testing.T) {
	messageSendCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		form, ok := requireVKRequest(t, writer, request, method)
		if !ok {
			return
		}
		switch method {
		case "likes.add", "likes.delete":
			if form.Get("type") != "post" || form.Get("owner_id") != "-456" || form.Get("item_id") != "7" {
				t.Errorf("%s form=%v", method, form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"likes": 3}})
		case "wall.createComment":
			if form.Get("owner_id") != "-456" || form.Get("post_id") != "7" || form.Get("message") != "reply" || form.Get("from_group") != "1" || form.Get("reply_to_comment") != "11" {
				t.Errorf("wall.createComment form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"comment_id": 12}})
		case "wall.deleteComment":
			if form.Get("owner_id") != "-456" || form.Get("comment_id") != "12" {
				t.Errorf("wall.deleteComment form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": 1})
		case "messages.send":
			messageSendCalls++
			if form.Get("peer_id") != "2000000001" || form.Get("message") != "hello" || form.Get("attachment") != "photo-456_1,doc-456_2_key" || form.Get("reply_to") != "4" || form.Get("group_id") != "456" || form.Get("random_id") != "12345" {
				t.Errorf("messages.send form=%v", form)
			}
			if messageSendCalls == 1 {
				writeTestJSON(t, writer, map[string]any{"response": 50})
			} else {
				writeTestJSON(t, writer, map[string]any{"response": map[string]any{"message_id": 51}})
			}
		case "messages.getById":
			if form.Get("message_ids") != "50" || form.Get("group_id") != "456" || form.Get("extended") != "0" {
				t.Errorf("messages.getById form=%v", form)
			}
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"count": 1, "items": []any{map[string]any{
				"id": 50, "conversation_message_id": 9, "date": testNow.Unix(), "from_id": 123, "peer_id": 2000000001, "text": "received", "out": 0,
				"reply_message": map[string]any{"id": 4},
				"attachments":   []any{map[string]any{"type": "photo", "photo": map[string]any{"id": 1, "owner_id": -456}}},
			}}}})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, user := newTestAdapter(t, server, TokenUser, -456, false)
	_, community := newTestAdapter(t, server, TokenCommunity, -456, false)

	reaction := socialhub.ReactionRequest{TargetID: "wall-456_7", Kind: socialhub.ReactionLike}
	if err := user.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := user.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	parent := "-456_11"
	comment, err := community.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "-456_7", ParentID: &parent, Text: "reply"})
	if err != nil || comment.ID != "-456_12" || comment.PostID != "-456_7" || comment.ParentID == nil || *comment.ParentID != parent || comment.AuthorID == nil || *comment.AuthorID != "-456" || comment.CreatedAt == nil || len(comment.Extensions) != 1 {
		t.Fatalf("comment=%#v error=%v", comment, err)
	}
	if err := user.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}

	text, reply := "hello", "4"
	input := socialhub.SendMessageRequest{
		ConversationID: "2000000001", Text: &text, ReplyToID: &reply,
		MediaIDs: []string{"photo-456_1", "doc-456_2_key"},
	}
	message, err := community.SendMessage(context.Background(), input, socialhub.WithIdempotencyKey("12345"))
	if err != nil || message.ID != "50" || message.ConversationID != input.ConversationID || message.Text == nil || *message.Text != text || message.ReplyToID == nil || *message.ReplyToID != reply || len(message.RecipientIDs) != 1 || message.SentAt == nil || message.Direction != socialhub.DirectionOutbound {
		t.Fatalf("sent message=%#v error=%v", message, err)
	}
	message, err = community.SendMessage(context.Background(), input, socialhub.WithIdempotencyKey("12345"))
	if err != nil || message.ID != "51" {
		t.Fatalf("object send response=%#v error=%v", message, err)
	}
	message, err = community.GetMessage(context.Background(), "50")
	if err != nil || message.ID != "50" || message.SenderID == nil || *message.SenderID != "123" || message.Text == nil || *message.Text != "received" || message.ReplyToID == nil || *message.ReplyToID != "4" || message.SentAt == nil || message.Direction != socialhub.DirectionInbound || len(message.Media) != 1 || len(message.Extensions) != 1 {
		t.Fatalf("received message=%#v error=%v", message, err)
	}
}

func TestReactionCommentAndMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, user := newTestAdapter(t, server, TokenUser, 123, false)
	_, community := newTestAdapter(t, server, TokenCommunity, -456, false)
	_, service := newTestAdapter(t, server, TokenService, 789, false)
	text, blank, badReply := "hello", " ", "bad"
	tooMany := make([]string, 11)
	for index := range tooMany {
		tooMany[index] = "photo123_" + strconv.Itoa(index+1)
	}
	invalid := []func() error{
		func() error {
			return user.React(context.Background(), socialhub.ReactionRequest{TargetID: "123_1", Kind: socialhub.ReactionRepost})
		},
		func() error {
			return community.React(context.Background(), socialhub.ReactionRequest{TargetID: "-456_1", Kind: socialhub.ReactionLike})
		},
		func() error {
			return user.React(context.Background(), socialhub.ReactionRequest{TargetID: "bad", Kind: socialhub.ReactionLike})
		},
		func() error {
			_, err := service.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "123_1", Text: "x"})
			return err
		},
		func() error {
			_, err := user.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "bad", Text: "x"})
			return err
		},
		func() error {
			_, err := user.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "123_1", Text: blank})
			return err
		},
		func() error {
			parent := "456_2"
			_, err := user.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "123_1", ParentID: &parent, Text: "x"})
			return err
		},
		func() error { return community.DeleteComment(context.Background(), "-456_1") },
		func() error {
			_, err := service.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", Text: &text})
			return err
		},
		func() error {
			_, err := user.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "bad", Text: &text})
			return err
		},
		func() error {
			_, err := user.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", RecipientIDs: []string{"2"}, Text: &text})
			return err
		},
		func() error {
			_, err := user.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", Text: &blank})
			return err
		},
		func() error {
			_, err := user.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", Text: &text, MediaIDs: tooMany})
			return err
		},
		func() error {
			_, err := user.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", Text: &text, MediaIDs: []string{"bad"}})
			return err
		},
		func() error {
			_, err := user.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", Text: &text, ReplyToID: &badReply})
			return err
		},
		func() error {
			_, err := user.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", Text: &text}, socialhub.WithIdempotencyKey("0"))
			return err
		},
		func() error { _, err := service.GetMessage(context.Background(), "1"); return err },
		func() error { _, err := user.GetMessage(context.Background(), "bad"); return err },
	}
	for index, call := range invalid {
		if err := call(); err == nil {
			t.Fatalf("validation %d accepted", index)
		}
	}
	if randomID, err := vkRandomID(""); err != nil || randomID <= 0 || randomID > int64(^uint32(0)>>1) {
		t.Fatalf("generated random_id=%d error=%v", randomID, err)
	}
	_, err := decodeMessageID([]byte(`false`))
	requireErrorCode(t, err, socialhub.CodePlatformError)
}

func TestMalformedInteractionResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/method/")
		if _, ok := requireVKRequest(t, writer, request, method); !ok {
			return
		}
		switch method {
		case "wall.createComment":
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"comment_id": 0}})
		case "wall.deleteComment":
			writeTestJSON(t, writer, map[string]any{"response": 0})
		case "messages.getById":
			writeTestJSON(t, writer, map[string]any{"response": map[string]any{"count": 0, "items": []any{}}})
		case "messages.send":
			writeTestJSON(t, writer, map[string]any{"response": false})
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, TokenUser, 123, false)
	text := "x"
	_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "123_1", Text: "x"})
	requireErrorCode(t, err, socialhub.CodePlatformError)
	err = client.DeleteComment(context.Background(), "123_1")
	requireErrorCode(t, err, socialhub.CodePlatformError)
	if _, err := client.GetMessage(context.Background(), "1"); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("get message response=%v", err)
	}
	_, err = client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "1", Text: &text}, socialhub.WithIdempotencyKey("1"))
	requireErrorCode(t, err, socialhub.CodePlatformError)
}
