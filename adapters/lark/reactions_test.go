package lark

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func reactionValue(id, emoji, actor string) map[string]any {
	return map[string]any{
		"reaction_id": id, "action_time": "1785571200000",
		"operator":      map[string]any{"operator_id": actor, "operator_type": "user"},
		"reaction_type": map[string]string{"emoji_type": emoji},
	}
}

func TestReactionAndCommentContracts(t *testing.T) {
	listPage := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID+"/reactions":
			body := requireLarkRequest(t, writer, request, TokenTenant)
			if body == nil {
				return
			}
			reactionType, _ := body["reaction_type"].(map[string]any)
			emoji, _ := reactionType["emoji_type"].(string)
			if emoji == "" {
				t.Errorf("reaction body=%v", body)
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": reactionValue(testReactionID, emoji, testActorID)})
		case request.Method == http.MethodGet && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID+"/reactions":
			if requireLarkRequest(t, writer, request, TokenTenant) == nil {
				return
			}
			if request.URL.Query().Get("reaction_type") != commonLikeEmoji || request.URL.Query().Get("page_size") != "50" {
				t.Errorf("reaction query=%v", request.URL.Query())
			}
			listPage++
			if listPage == 1 {
				writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{
					"items": []any{reactionValue("reaction_other", commonLikeEmoji, "ou_other")}, "has_more": true, "page_token": "next",
				}})
				return
			}
			if request.URL.Query().Get("page_token") != "next" {
				t.Errorf("second reaction query=%v", request.URL.Query())
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{
				"items": []any{reactionValue(testReactionID, commonLikeEmoji, testActorID)}, "has_more": false,
			}})
		case request.Method == http.MethodDelete && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID+"/reactions/"+testReactionID:
			if requireLarkRequest(t, writer, request, TokenTenant) == nil {
				return
			}
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{}})
		case request.Method == http.MethodPost && request.URL.Path == "/open-apis/im/v1/messages/"+testMessageID+"/reply":
			body := requireLarkRequest(t, writer, request, TokenTenant)
			if body == nil {
				return
			}
			if body["msg_type"] != "text" || body["content"] != `{"text":"a reply"}` {
				t.Errorf("comment body=%v", body)
			}
			value := wireMessageValue(testReplyID, "a reply")
			value["root_id"], value["parent_id"] = testMessageID, testMessageID
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": value})
		case request.Method == http.MethodDelete && request.URL.Path == "/open-apis/im/v1/messages/"+testReplyID:
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
	ctx := context.Background()

	reaction, err := client.AddReaction(ctx, testMessageID, "EYES")
	if err != nil || reaction.ID != testReactionID || reaction.EmojiType != "EYES" || reaction.ActorID != testActorID {
		t.Fatalf("reaction=%#v err=%v", reaction, err)
	}
	if err := client.DeleteReaction(ctx, testMessageID, testReactionID); err != nil {
		t.Fatal(err)
	}
	if err := client.React(ctx, socialhub.ReactionRequest{ActorID: testActorID, TargetID: testMessageID, Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(ctx, socialhub.ReactionRequest{TargetID: testMessageID, Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	parentID := testMessageID
	comment, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: testMessageID, ParentID: &parentID, Text: "a reply"})
	if err != nil || comment.ID != testReplyID || comment.ParentID == nil || *comment.ParentID != testMessageID || comment.Text != "a reply" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(ctx, testReplyID); err != nil {
		t.Fatal(err)
	}
}

func TestReactionValidationAndNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			writeTestJSON(t, writer, map[string]any{"code": 0, "data": map[string]any{"items": []any{}, "has_more": false}})
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()
	_, client := newTestClient(t, server, TokenTenant, testChatID, testActorID, false)
	ctx := context.Background()

	if _, err := client.AddReaction(ctx, "bad/id", "EYES"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid add=%v", err)
	}
	if _, err := client.AddReaction(ctx, testMessageID, "EYES", socialhub.WithFields("id")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("add options=%v", err)
	}
	if err := client.DeleteReaction(ctx, testMessageID, "bad/id"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid delete=%v", err)
	}
	if err := client.React(ctx, socialhub.ReactionRequest{TargetID: testMessageID, Kind: socialhub.ReactionRepost}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unsupported react=%v", err)
	}
	if err := client.React(ctx, socialhub.ReactionRequest{ActorID: "ou_other", TargetID: testMessageID, Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("actor mismatch=%v", err)
	}
	if err := client.RemoveReaction(ctx, socialhub.ReactionRequest{TargetID: testMessageID, Kind: socialhub.ReactionRepost}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unsupported remove=%v", err)
	}
	if err := client.RemoveReaction(ctx, socialhub.ReactionRequest{ActorID: "ou_other", TargetID: testMessageID, Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("remove actor mismatch=%v", err)
	}
	if err := client.RemoveReaction(ctx, socialhub.ReactionRequest{TargetID: "bad/id", Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("remove target=%v", err)
	}
	if err := client.RemoveReaction(ctx, socialhub.ReactionRequest{TargetID: testMessageID, Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrNotFound) {
		t.Fatalf("missing reaction=%v", err)
	}
	if _, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: testMessageID}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty comment=%v", err)
	}
	if _, err := client.Comment(ctx, socialhub.CreateCommentRequest{PostID: testMessageID, Text: "x"}, socialhub.WithFields("id")); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comment options=%v", err)
	}

	encoded, _ := json.Marshal(map[string]string{"text": "x"})
	if string(encoded) != `{"text":"x"}` {
		t.Fatal("unexpected JSON encoding")
	}
}
