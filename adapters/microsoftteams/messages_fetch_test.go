package microsoftteams

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func writeTestJSON(t *testing.T, writer http.ResponseWriter, status int, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func testMessage(id, replyTo, content string) map[string]any {
	message := map[string]any{
		"id": id, "etag": "etag-1", "messageType": "message", "chatId": testChatID,
		"createdDateTime": testNow.Format(time.RFC3339), "lastModifiedDateTime": testNow.Add(time.Minute).Format(time.RFC3339),
		"webUrl": "https://teams.microsoft.com/l/message/" + id, "importance": "normal", "locale": "en-us",
		"from":           map[string]any{"user": map[string]any{"id": testActorID, "displayName": "Ada"}},
		"body":           map[string]any{"contentType": "text", "content": content},
		"hostedContents": []map[string]any{{"id": "hosted-1", "contentType": "image/png"}},
		"attachments":    []map[string]any{{"id": "file-1", "name": "doc.pdf", "contentType": "application/pdf", "contentUrl": "https://contoso.example/doc.pdf"}},
		"reactions":      []map[string]any{{"reactionType": "like", "createdDateTime": testNow.Format(time.RFC3339), "user": map[string]any{"user": map[string]any{"id": testActorID}}}},
	}
	if replyTo != "" {
		message["replyToId"] = replyTo
	}
	return message
}

func graphContractHandler(t *testing.T, serverURL *string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		path := request.URL.Path
		switch {
		case request.Method == http.MethodPost && path == "/v1.0/chats/"+testChatID+"/messages":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode send: %v", err)
			}
			if body["body"] == nil {
				t.Errorf("send body=%#v", body)
			}
			writeTestJSON(t, writer, http.StatusCreated, testMessage(testRootID, "", "hello"))
		case request.Method == http.MethodPost && path == "/v1.0/chats/"+testChatID+"/messages/"+testRootID+"/replies":
			writeTestJSON(t, writer, http.StatusCreated, testMessage(testReplyID, testRootID, "reply"))
		case request.Method == http.MethodGet && path == "/v1.0/chats/"+testChatID+"/messages/"+testRootID:
			writeTestJSON(t, writer, http.StatusOK, testMessage(testRootID, "", "hello"))
		case request.Method == http.MethodGet && path == "/v1.0/chats/"+testChatID+"/messages/"+testRootID+"/replies/"+testReplyID:
			writeTestJSON(t, writer, http.StatusOK, testMessage(testReplyID, testRootID, "reply"))
		case request.Method == http.MethodGet && path == "/v1.0/chats/"+testChatID+"/messages" && request.URL.Query().Get("$skiptoken") == "next":
			writeTestJSON(t, writer, http.StatusOK, map[string]any{"value": []any{testMessage("root-2", "", "second")}})
		case request.Method == http.MethodGet && path == "/v1.0/chats/"+testChatID+"/messages":
			if request.URL.Query().Get("$top") == "" {
				t.Errorf("missing $top: %s", request.URL.RawQuery)
			}
			writeTestJSON(t, writer, http.StatusOK, map[string]any{
				"value":           []any{testMessage(testRootID, "", "hello")},
				"@odata.nextLink": *serverURL + "/v1.0/chats/" + testChatID + "/messages?$skiptoken=next",
			})
		case request.Method == http.MethodGet && path == "/v1.0/chats/"+testChatID+"/messages/"+testRootID+"/replies":
			writeTestJSON(t, writer, http.StatusOK, map[string]any{"value": []any{testMessage(testReplyID, testRootID, "reply")}})
		case request.Method == http.MethodPatch && path == "/v1.0/chats/"+testChatID+"/messages/"+testRootID:
			writeTestJSON(t, writer, http.StatusOK, testMessage(testRootID, "", "edited"))
		case request.Method == http.MethodPost && (strings.HasSuffix(path, "/softDelete") || strings.HasSuffix(path, "/setReaction") || strings.HasSuffix(path, "/unsetReaction")):
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPost && path == "/v1.0/subscriptions":
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body["includeResourceData"] != false || body["clientState"] != "client-state" || body["changeType"] != "created,updated" {
				t.Errorf("subscription body=%#v", body)
			}
			writeTestJSON(t, writer, http.StatusCreated, map[string]any{"id": "subscription-1", "resource": body["resource"], "changeType": body["changeType"], "expirationDateTime": body["expirationDateTime"], "includeResourceData": false})
		case request.Method == http.MethodPatch && path == "/v1.0/subscriptions/subscription-1":
			writeTestJSON(t, writer, http.StatusOK, map[string]any{"id": "subscription-1", "expirationDateTime": testNow.Add(30 * time.Minute).Format(time.RFC3339), "includeResourceData": false})
		case request.Method == http.MethodDelete && path == "/v1.0/subscriptions/subscription-1":
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s?%s", request.Method, path, request.URL.RawQuery)
			writer.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestTypedMessageFetchReactionAndSubscriptionContracts(t *testing.T) {
	var serverURL string
	_, client, server := newTestAdapter(t, graphContractHandler(t, &serverURL), CloudGlobal, TokenDelegated, true, true, allTestScopes())
	serverURL = server.URL
	target := Target{Kind: TargetChat, ChatID: testChatID}
	root := MessageRef{Target: target, RootID: testRootID}
	reply := MessageRef{Target: target, RootID: testRootID, ReplyID: testReplyID}

	message, err := client.Send(context.Background(), SendRequest{
		Target: target, Body: MessageBody{ContentType: "html", Content: `<p>hello<img src="../hostedContents/1/$value"></p>`},
		Importance: "high", HostedContents: []HostedContent{{TemporaryID: "1", ContentType: "image/png", ContentBytes: []byte{1, 2, 3}}},
	}, socialhub.WithRequestID("client-request"))
	if err != nil || message.ID != testRootID || len(message.HostedContents) != 1 || len(message.Reactions) != 1 || len(message.Raw) == 0 {
		t.Fatalf("send=%#v error=%v", message, err)
	}
	message, err = client.Reply(context.Background(), ReplyRequest{Parent: root, Body: MessageBody{ContentType: "text", Content: "reply"}})
	if err != nil || message.ID != testReplyID || message.ReplyToID != testRootID {
		t.Fatalf("reply=%#v error=%v", message, err)
	}
	message, err = client.Get(context.Background(), reply)
	if err != nil || message.ID != testReplyID {
		t.Fatalf("get=%#v error=%v", message, err)
	}
	message, err = client.Update(context.Background(), UpdateRequest{Message: root, Body: MessageBody{ContentType: "text", Content: "edited"}})
	if err != nil || message.ID != testRootID {
		t.Fatalf("update=%#v error=%v", message, err)
	}
	if err := client.SoftDelete(context.Background(), reply); err != nil {
		t.Fatalf("soft delete=%v", err)
	}
	page, err := client.List(context.Background(), ListMessagesRequest{Target: target, MaxResults: 2})
	if err != nil || len(page.Items) != 1 || !page.HasMore || page.NextCursor == "" {
		t.Fatalf("page=%#v error=%v", page, err)
	}
	next, err := client.List(context.Background(), ListMessagesRequest{Target: target, Cursor: page.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].ID != "root-2" || next.HasMore {
		t.Fatalf("next=%#v error=%v", next, err)
	}
	replies, err := client.ListReplies(context.Background(), ListRepliesRequest{Parent: root})
	if err != nil || len(replies.Items) != 1 || replies.Items[0].ID != testReplyID {
		t.Fatalf("replies=%#v error=%v", replies, err)
	}
	if err := client.SetReaction(context.Background(), root, "heart"); err != nil {
		t.Fatalf("set reaction=%v", err)
	}
	if err := client.UnsetReaction(context.Background(), reply, "❤️"); err != nil {
		t.Fatalf("unset reaction=%v", err)
	}

	subscription, err := client.CreateSubscription(context.Background(), CreateSubscriptionRequest{
		Resource: "/chats/" + testChatID + "/messages", ChangeTypes: []string{"created", "updated", "created"},
		NotificationURL: "https://hooks.example/graph", ExpirationDateTime: testNow.Add(30 * time.Minute),
	})
	if err != nil || subscription.ID != "subscription-1" || subscription.IncludeResourceData {
		t.Fatalf("subscription=%#v error=%v", subscription, err)
	}
	renewed, err := client.RenewSubscription(context.Background(), "subscription-1", testNow.Add(30*time.Minute))
	if err != nil || renewed.ID != "subscription-1" {
		t.Fatalf("renewed=%#v error=%v", renewed, err)
	}
	if err := client.DeleteSubscription(context.Background(), "subscription-1"); err != nil {
		t.Fatalf("delete subscription=%v", err)
	}
}

func TestCommonMessagePublisherFetcherAndReactor(t *testing.T) {
	var serverURL string
	_, client, server := newTestAdapter(t, graphContractHandler(t, &serverURL), CloudGlobal, TokenDelegated, true, true, allTestScopes())
	serverURL = server.URL
	target := Target{Kind: TargetChat, ChatID: testChatID}
	conversationID, _ := ConversationRef(target)
	rootID, _ := EncodeMessageRef(MessageRef{Target: target, RootID: testRootID})
	replyID, _ := EncodeMessageRef(MessageRef{Target: target, RootID: testRootID, ReplyID: testReplyID})
	text := "hello"

	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: conversationID, Text: &text})
	if err != nil || message.ID != rootID || message.Direction != socialhub.DirectionOutbound || len(message.Media) != 2 {
		t.Fatalf("message=%#v error=%v", message, err)
	}
	message, err = client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: conversationID, Text: &text, ReplyToID: &rootID})
	if err != nil || message.ID != replyID || message.ReplyToID == nil || *message.ReplyToID != rootID {
		t.Fatalf("reply message=%#v error=%v", message, err)
	}
	message, err = client.GetMessage(context.Background(), rootID)
	if err != nil || message.ID != rootID || message.Direction != socialhub.DirectionOutbound {
		t.Fatalf("get message=%#v error=%v", message, err)
	}
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if err != nil || post.ID != rootID || post.Status == nil || post.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("post=%#v error=%v", post, err)
	}
	post, err = client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, ReplyToID: &rootID})
	if err != nil || post.ID != replyID || len(post.Relations) != 1 {
		t.Fatalf("reply post=%#v error=%v", post, err)
	}
	post, err = client.GetPost(context.Background(), rootID)
	if err != nil || post.ID != rootID {
		t.Fatalf("get post=%#v error=%v", post, err)
	}
	status, err := client.PublishStatus(context.Background(), rootID)
	if err != nil || status.State != socialhub.PublishStatePublished {
		t.Fatalf("status=%#v error=%v", status, err)
	}
	if err := client.DeletePost(context.Background(), rootID); err != nil {
		t.Fatalf("delete post=%v", err)
	}
	posts, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{MaxResults: 1})
	if err != nil || len(posts.Items) != 1 || posts.NextCursor == nil || !posts.HasMore {
		t.Fatalf("posts=%#v error=%v", posts, err)
	}
	comments, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{PostID: rootID})
	if err != nil || len(comments.Items) != 1 || comments.Items[0].ID != replyID {
		t.Fatalf("comments=%#v error=%v", comments, err)
	}
	if err := client.React(context.Background(), socialhub.ReactionRequest{ActorID: testActorID, TargetID: rootID, Kind: socialhub.ReactionLike}); err != nil {
		t.Fatalf("react=%v", err)
	}
	if err := client.RemoveReaction(context.Background(), socialhub.ReactionRequest{TargetID: replyID, Kind: socialhub.ReactionLike}); err != nil {
		t.Fatalf("remove reaction=%v", err)
	}
	comment, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: rootID, Text: "reply"})
	if err != nil || comment.ID != replyID || comment.PostID != rootID {
		t.Fatalf("comment=%#v error=%v", comment, err)
	}
	if err := client.DeleteComment(context.Background(), replyID); err != nil {
		t.Fatalf("delete comment=%v", err)
	}
	if _, err := client.GetUser(context.Background(), testActorID); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get user=%v", err)
	}
}

func TestMessageAndWorkflowValidation(t *testing.T) {
	_, client, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenDelegated, true, true, allTestScopes())
	target := Target{Kind: TargetChat, ChatID: testChatID}
	root := MessageRef{Target: target, RootID: testRootID}
	rootID, _ := EncodeMessageRef(root)
	conversation, _ := ConversationRef(target)
	text, visibility, quote := "hello", "public", rootID
	huge := make([]byte, (4<<20)+1)
	parent := "nested"
	invalid := []func() error{
		func() error { _, err := client.Send(context.Background(), SendRequest{}); return err },
		func() error {
			_, err := client.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "markdown", Content: text}})
			return err
		},
		func() error {
			_, err := client.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: " "}})
			return err
		},
		func() error {
			_, err := client.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: text}, Importance: "critical"})
			return err
		},
		func() error {
			_, err := client.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: text}, HostedContents: []HostedContent{{TemporaryID: "1", ContentType: "image/png", ContentBytes: huge}}})
			return err
		},
		func() error {
			_, err := client.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: text}, HostedContents: []HostedContent{{TemporaryID: "1", ContentType: "image/png", ContentBytes: []byte{1}}, {TemporaryID: "1", ContentType: "image/png", ContentBytes: []byte{2}}}})
			return err
		},
		func() error {
			_, err := client.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: text}}, socialhub.WithFields("id"))
			return err
		},
		func() error {
			_, err := client.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: text}}, socialhub.WithIdempotencyKey("key"))
			return err
		},
		func() error {
			_, err := client.Reply(context.Background(), ReplyRequest{Parent: MessageRef{Target: target, RootID: testRootID, ReplyID: testReplyID}, Body: MessageBody{ContentType: "text", Content: text}})
			return err
		},
		func() error {
			_, err := client.List(context.Background(), ListMessagesRequest{Target: target, MaxResults: -1})
			return err
		},
		func() error {
			_, err := client.List(context.Background(), ListMessagesRequest{Target: target, Cursor: "https://evil.example/v1.0/chats/x/messages"})
			return err
		},
		func() error {
			_, err := client.List(context.Background(), ListMessagesRequest{Target: target, Cursor: "relative"})
			return err
		},
		func() error {
			_, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: conversation, Text: &text, RecipientIDs: []string{"user"}})
			return err
		},
		func() error {
			_, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: conversation, Text: &text, MediaIDs: []string{"file"}})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, Visibility: &visibility})
			return err
		},
		func() error {
			_, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text, QuotePostID: &quote})
			return err
		},
		func() error {
			_, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{StartTime: &testNow})
			return err
		},
		func() error {
			return client.React(context.Background(), socialhub.ReactionRequest{TargetID: rootID, Kind: socialhub.ReactionRepost})
		},
		func() error { return client.SetReaction(context.Background(), root, "\n") },
		func() error {
			_, err := client.Comment(context.Background(), socialhub.CreateCommentRequest{PostID: rootID, Text: text, ParentID: &parent})
			return err
		},
		func() error { return client.DeleteComment(context.Background(), rootID) },
		func() error {
			_, err := client.CreateSubscription(context.Background(), CreateSubscriptionRequest{})
			return err
		},
		func() error {
			_, err := client.RenewSubscription(context.Background(), "bad/id", testNow.Add(time.Minute))
			return err
		},
		func() error { return client.DeleteSubscription(context.Background(), "bad/id") },
	}
	for index, call := range invalid {
		if err := call(); err == nil {
			t.Fatalf("validation %d accepted", index)
		}
	}
}

func TestTokenKindScopeAndCloudRestrictions(t *testing.T) {
	_, application, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenApplication, false, false, nil)
	target := Target{Kind: TargetChat, ChatID: testChatID}
	root := MessageRef{Target: target, RootID: testRootID}
	if _, err := application.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: "hello"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("application send=%v", err)
	}
	if err := application.SetReaction(context.Background(), root, "like"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("application reaction=%v", err)
	}

	_, china, _ := newTestAdapter(t, http.NotFoundHandler(), CloudChina, TokenDelegated, true, false, allTestScopes())
	if _, err := china.Update(context.Background(), UpdateRequest{Message: root, Body: MessageBody{ContentType: "text", Content: "edited"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("China edit=%v", err)
	}

	_, missingScope, _ := newTestAdapter(t, http.NotFoundHandler(), CloudGlobal, TokenDelegated, true, false, []string{"User.Read"})
	if _, err := missingScope.Get(context.Background(), root); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing read scope=%v", err)
	}
	if _, err := missingScope.Send(context.Background(), SendRequest{Target: target, Body: MessageBody{ContentType: "text", Content: "hello"}}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing send scope=%v", err)
	}
	if err := missingScope.SetReaction(context.Background(), root, "like"); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("missing reaction scope=%v", err)
	}
}

func TestMalformedGraphMessageAndReplyPage(t *testing.T) {
	mode := "message"
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch mode {
		case "message":
			_, _ = io.WriteString(writer, `{"body":{"contentType":"text","content":"missing id"}}`)
		case "reply":
			writeTestJSON(t, writer, http.StatusOK, map[string]any{"value": []any{testMessage(testReplyID, "other-root", "reply")}})
		}
	})
	_, client, _ := newTestAdapter(t, handler, CloudGlobal, TokenDelegated, true, false, allTestScopes())
	target := Target{Kind: TargetChat, ChatID: testChatID}
	if _, err := client.Get(context.Background(), MessageRef{Target: target, RootID: testRootID}); err == nil {
		t.Fatal("missing message ID accepted")
	}
	mode = "reply"
	if _, err := client.ListReplies(context.Background(), ListRepliesRequest{Parent: MessageRef{Target: target, RootID: testRootID}}); err == nil {
		t.Fatal("foreign reply root accepted")
	}
}
