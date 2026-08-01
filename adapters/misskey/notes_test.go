package misskey

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestPublishReactionAndCommentContracts(t *testing.T) {
	createCalls, deleteCalls, reactionCalls := 0, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer access-token" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode %s: %v", request.URL.Path, err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/api/notes/create":
			createCalls++
			switch createCalls {
			case 1:
				if body["text"] != "common" || body["visibility"] != "home" || body["renoteId"] != "quoted" ||
					len(body["fileIds"].([]any)) != 1 {
					t.Errorf("common create=%v", body)
				}
			case 2:
				poll := body["poll"].(map[string]any)
				if body["visibility"] != "specified" || body["cw"] != "CW" || body["localOnly"] != true ||
					body["reactionAcceptance"] != "likeOnly" || body["channelId"] != "channel" ||
					len(body["visibleUserIds"].([]any)) != 1 || poll["expiredAfter"] != float64(time.Minute.Milliseconds()) {
					t.Errorf("typed create=%v", body)
				}
			case 3:
				if body["text"] != "reply" || body["replyId"] != "parent" {
					t.Errorf("comment create=%v", body)
				}
			default:
				t.Errorf("unexpected create call %d", createCalls)
			}
			note := testNote("created-"+string(rune('0'+createCalls)), "created")
			writeTestJSON(t, writer, map[string]any{"createdNote": note})
		case "/api/notes/show":
			writeTestJSON(t, writer, testNote(body["noteId"].(string), "status"))
		case "/api/notes/delete":
			deleteCalls++
			writer.WriteHeader(http.StatusNoContent)
		case "/api/notes/reactions/create":
			reactionCalls++
			if body["noteId"] != "note" {
				t.Errorf("reaction body=%v", body)
			}
			if reactionCalls == 1 && body["reaction"] != ":thumbsup:" {
				t.Errorf("default reaction=%v", body)
			}
			if reactionCalls == 2 && body["reaction"] != ":party:" {
				t.Errorf("typed reaction=%v", body)
			}
			writer.WriteHeader(http.StatusNoContent)
		case "/api/notes/reactions/delete":
			if body["noteId"] != "note" {
				t.Errorf("remove body=%v", body)
			}
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, allTestPermissions())

	commonText, visibility, quote := "common", "home", "quoted"
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{
		Text: &commonText, MediaIDs: []string{"file"}, QuotePostID: &quote, Visibility: &visibility,
	})
	if err != nil || post.ID != "created-1" {
		t.Fatalf("common post=%#v err=%v", post, err)
	}
	typedText, warning, replyID, channelID := "typed\nsecond line", "CW", "root", "channel"
	acceptance := ReactionLikeOnly
	typed, err := client.CreateNote(context.Background(), CreateNoteRequest{
		Text: &typedText, ReplyID: &replyID, Visibility: VisibilitySpecified,
		VisibleUserIDs: []string{"recipient"}, ContentWarning: &warning, LocalOnly: true,
		ReactionAcceptance: &acceptance, ChannelID: &channelID,
		Poll: &Poll{Choices: []string{"yes", "no"}, Multiple: true, ExpireAfter: time.Minute},
	})
	if err != nil || typed.ID != "created-2" {
		t.Fatalf("typed post=%#v err=%v", typed, err)
	}
	status, err := client.PublishStatus(context.Background(), typed.ID)
	if err != nil || status.ID != typed.ID || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: "user-1", TargetID: "note", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.ReactWithEmoji(context.Background(), "note", ":party:"); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: "note", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	parent := "parent"
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "root", ParentID: &parent, Text: "reply"})
	if err != nil || comment.ID != "created-3" || comment.PostID != "root" || comment.ParentID == nil || *comment.ParentID != "parent" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeletePost(context.Background(), post.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
	if createCalls != 3 || deleteCalls != 2 || reactionCalls != 2 {
		t.Fatalf("calls create=%d delete=%d reactions=%d", createCalls, deleteCalls, reactionCalls)
	}
}

func TestCreateNoteValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, allTestPermissions())
	pointer := func(value string) *string { return &value }
	badAcceptance := ReactionAcceptance("unknown")
	past := testNow.Add(-time.Minute)
	future := testNow.Add(time.Hour)
	manyFiles := make([]string, 17)
	for index := range manyFiles {
		manyFiles[index] = "file-" + string(rune('a'+index))
	}
	validText := pointer("text")
	invalid := []CreateNoteRequest{
		{},
		{Text: pointer("")},
		{Text: validText, Visibility: "direct"},
		{Text: validText, FileIDs: []string{"file", "file"}},
		{Text: validText, FileIDs: manyFiles},
		{Text: validText, VisibleUserIDs: []string{"user"}},
		{Text: validText, Visibility: VisibilitySpecified},
		{Text: validText, Visibility: VisibilitySpecified, VisibleUserIDs: []string{"user", "user"}},
		{Text: validText, ReplyID: pointer(" bad")},
		{Text: validText, ContentWarning: pointer("")},
		{Text: validText, ReactionAcceptance: &badAcceptance},
		{Poll: &Poll{Choices: []string{"only"}}},
		{Poll: &Poll{Choices: []string{"same", "same"}}},
		{Poll: &Poll{Choices: []string{"yes", "no"}, ExpiresAt: &past}},
		{Poll: &Poll{Choices: []string{"yes", "no"}, ExpiresAt: &future, ExpireAfter: time.Second}},
		{Poll: &Poll{Choices: []string{"yes", "no"}, ExpireAfter: -time.Second}},
	}
	for index, input := range invalid {
		if _, err := client.CreateNote(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid note %d=%v input=%#v", index, err, input)
		}
	}
	if _, err := client.Publish(context.Background(), socialhub.CreatePostRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid publish=%v", err)
	}
	if err := client.DeletePost(context.Background(), ""); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("invalid delete=%v", err)
	}
}

func TestReactionAndCommentValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, allTestPermissions())
	invalidReactions := []socialhub.ReactionRequest{
		{TargetID: "note"},
		{TargetID: "note", ActorID: "other", Kind: socialhub.ReactionLike},
	}
	for index, input := range invalidReactions {
		if err := client.React(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid reaction %d=%v", index, err)
		}
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: "note", Kind: socialhub.ReactionRepost}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("repost=%v", err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: "note", Kind: socialhub.ReactionRepost}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("remove repost=%v", err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("remove invalid=%v", err)
	}
	if err := client.ReactWithEmoji(context.Background(), "", ":like:"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("emoji invalid=%v", err)
	}
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("comment invalid=%v", err)
	}
	badParent := " bad"
	if _, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "root", ParentID: &badParent, Text: "x"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("parent invalid=%v", err)
	}
}
