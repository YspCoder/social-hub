package discord

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

func TestBotMessagingFetchingAndReactionContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bot bot-token" || request.Header.Get("User-Agent") != "DiscordBot (test, 1)" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/users/@me":
			writeJSON(writer, `{"id":"10","username":"hub","global_name":"Hub Bot","avatar":"a_hash","bot":true}`)
		case request.Method == http.MethodGet && request.URL.Path == "/gateway/bot":
			writeJSON(writer, `{"url":"wss://gateway.discord.gg","shards":2,"session_start_limit":{"total":1000,"remaining":999,"reset_after":60000,"max_concurrency":1}}`)
		case request.Method == http.MethodGet && request.URL.Path == "/users/42":
			writeJSON(writer, `{"id":"42","username":"alice","global_name":"Alice","avatar":"hash"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/channels/100/messages":
			var input messageCreate
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.AllowedMentions.Parse == nil || len(input.AllowedMentions.Parse) != 0 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			id := "201"
			if input.Content == "reply" {
				id = "202"
			}
			if input.MessageReference != nil && (input.MessageReference.ChannelID != "100" || input.MessageReference.MessageID == "") {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `{"id":"`+id+`","channel_id":"100","guild_id":"500","author":{"id":"10","username":"hub","bot":true},"content":"`+input.Content+`","timestamp":"2026-08-01T00:00:00Z","message_reference":`+referenceJSON(input.MessageReference)+`}`)
		case request.Method == http.MethodGet && request.URL.Path == "/channels/100/messages/201":
			writeJSON(writer, `{"id":"201","channel_id":"100","guild_id":"500","author":{"id":"10","username":"hub","bot":true},"content":"hello","timestamp":"2026-08-01T00:00:00Z","attachments":[{"id":"700","filename":"image.png","content_type":"image/png","size":12,"url":"https://cdn.example/image.png","width":10,"height":20}]}`)
		case request.Method == http.MethodGet && request.URL.Path == "/channels/100/messages":
			if request.URL.Query().Get("limit") != "2" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(writer, `[{"id":"300","channel_id":"100","guild_id":"500","author":{"id":"42","username":"alice"},"content":"new","timestamp":"2026-08-01T00:00:00Z"},{"id":"299","channel_id":"100","guild_id":"500","author":{"id":"42","username":"alice"},"content":"old","timestamp":"2026-07-31T23:00:00Z"}]`)
		case (request.Method == http.MethodPut || request.Method == http.MethodDelete) && strings.HasSuffix(request.URL.Path, "/reactions/👍/@me"):
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/channels/100/messages/"):
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server, "100")

	current, err := client.BotWorkflow().CurrentUser(context.Background())
	if err != nil || current.ID != "10" || current.AccountType == nil || *current.AccountType != "bot" || current.AvatarURL == nil || !strings.HasSuffix(*current.AvatarURL, ".gif") {
		t.Fatalf("current=%#v err=%v", current, err)
	}
	gateway, err := client.BotWorkflow().Gateway(context.Background())
	if err != nil || gateway.Shards != 2 || gateway.SessionStartLimit.Remaining != 999 {
		t.Fatalf("gateway=%#v err=%v", gateway, err)
	}
	user, err := client.GetUser(context.Background(), "42")
	if err != nil || user.DisplayName == nil || *user.DisplayName != "Alice" {
		t.Fatalf("user=%#v err=%v", user, err)
	}

	text := "hello"
	replyTo := "100/199"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "100", Text: &text, ReplyToID: &replyTo})
	if err != nil || message.ID != "100/201" || message.ReplyToID == nil || *message.ReplyToID != replyTo {
		t.Fatalf("message=%#v err=%v", message, err)
	}
	fetched, err := client.GetMessage(context.Background(), message.ID)
	if err != nil || fetched.ID != "100/201" || len(fetched.Media) != 1 || fetched.Media[0].Type != socialhub.MediaTypeImage {
		t.Fatalf("fetched=%#v err=%v", fetched, err)
	}
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if err != nil || post.ID != "100/201" || post.URL == nil || *post.URL != "https://discord.com/channels/500/100/201" {
		t.Fatalf("post=%#v err=%v", post, err)
	}
	page, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == nil || *page.NextCursor != "100/299" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: "100/201", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: "100/201", Kind: socialhub.ReactionLike}); err != nil {
		t.Fatal(err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: "100/201", Text: "reply"})
	if err != nil || comment.ID != "100/202" || comment.PostID != "100/201" {
		t.Fatalf("comment=%#v err=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), comment.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.DeletePost(context.Background(), post.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUnsupportedAndValidationBoundaries(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, "100")
	text := "hello"
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "100", Text: &text, MediaIDs: []string{"file"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("media error=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: "100/1"}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("comments error=%v", err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{TargetID: "bad", Kind: socialhub.ReactionLike}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reaction error=%v", err)
	}
	long := strings.Repeat("x", 2001)
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "100", Text: &long}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long message error=%v", err)
	}
}

func writeJSON(writer http.ResponseWriter, value string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(value))
}

func referenceJSON(reference *messageReference) string {
	if reference == nil {
		return "null"
	}
	encoded, _ := json.Marshal(reference)
	return string(encoded)
}
