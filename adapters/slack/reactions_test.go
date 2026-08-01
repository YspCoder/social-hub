package slack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestReactionAndThreadCommentContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		method := strings.TrimPrefix(request.URL.Path, "/api/")
		body := requireSlackRequest(t, writer, request, method)
		if body == nil {
			return
		}
		switch method {
		case "reactions.add", "reactions.remove":
			if body["name"] != "thumbsup" || body["channel"] != testChannelID || body["timestamp"] != testTimestamp {
				t.Errorf("%s body=%v", method, body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true})
		case "chat.postMessage":
			if body["channel"] != testChannelID || body["thread_ts"] != testTimestamp || body["text"] != "thread reply" {
				t.Errorf("comment body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{
				"ok": true, "channel": testChannelID, "ts": "1785571201.000002",
				"message": map[string]any{"type": "message", "user": testActorID, "text": "thread reply", "ts": "1785571201.000002", "thread_ts": testTimestamp},
			})
		case "chat.delete":
			if body["channel"] != testChannelID || body["ts"] != "1785571201.000002" {
				t.Errorf("delete comment body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"ok": true, "channel": testChannelID, "ts": "1785571201.000002"})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, false, allTestScopes())
	target := testChannelID + ":" + testTimestamp
	reaction := socialhub.ReactionRequest{ActorID: testActorID, TargetID: target, Kind: socialhub.ReactionLike}
	if err := client.React(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), reaction); err != nil {
		t.Fatal(err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: target, Text: "thread reply"})
	if err != nil || comment.ID != testChannelID+":1785571201.000002" || comment.PostID != target || comment.ParentID != nil || comment.Text != "thread reply" || comment.CreatedAt == nil || len(comment.Extensions) != 1 {
		t.Fatalf("comment=%#v error=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
}

func TestReactionAndCommentValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, testChannelID, false, allTestScopes())
	target := testChannelID + ":" + testTimestamp
	invalid := []func() error{
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: target, Kind: socialhub.ReactionRepost})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{ActorID: "U999ABC", TargetID: target, Kind: socialhub.ReactionLike})
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: "bad", Kind: socialhub.ReactionLike})
		},
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "bad", Text: "x"})
			return err
		},
		func() error {
			parent := testChannelID + ":1785571200.000099"
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: target, ParentID: &parent, Text: "x"})
			return err
		},
	}
	for index, call := range invalid {
		if err := call(); err == nil {
			t.Fatalf("validation %d accepted", index)
		}
	}
	_, restricted := newTestAdapter(t, server, testChannelID, false, []string{"chat:write"})
	if err := restricted.React(context.Background(), socialhub.ReactionRequest{TargetID: target, Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing reactions scope=%v", err)
	}
}
