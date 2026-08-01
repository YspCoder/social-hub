package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestSendMessagePublishAndDeleteContracts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse form: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/bot123456:bot-token/sendMessage":
			if request.FormValue("chat_id") != "-1001" || request.FormValue("text") == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if requests == 1 && request.FormValue("reply_parameters") != `{"message_id":41}` {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"message_id":42,"from":{"id":123456,"is_bot":true,"first_name":"Hub Bot","username":"hub_bot"},"date":1785542400,"chat":{"id":-1001,"type":"channel","username":"news"},"reply_to_message":{"message_id":41,"date":1785542300,"chat":{"id":-1001,"type":"channel"}},"text":"hello"}}`))
		case "/bot123456:bot-token/deleteMessage":
			if request.FormValue("chat_id") != "-1001" || request.FormValue("message_id") != "42" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"ok":true,"result":true}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, "-1001", true)
	text := "hello"
	reply := "41"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "-1001", Text: &text, ReplyToID: &reply})
	if err != nil {
		t.Fatal(err)
	}
	if message.ID != "42" || message.Direction != socialhub.DirectionOutbound || message.SenderID == nil || *message.SenderID != "123456" || len(message.RecipientIDs) != 1 || message.ReplyToID == nil || *message.ReplyToID != "41" {
		t.Fatalf("message=%#v", message)
	}
	post, err := client.Publish(context.Background(), socialhub.CreatePostRequest{Text: &text})
	if err != nil {
		t.Fatal(err)
	}
	if post.ID != "42" || post.URL == nil || *post.URL != "https://t.me/news/42" || post.Status == nil || post.Status.State != socialhub.PublishStatePublished {
		t.Fatalf("post=%#v", post)
	}
	if err := client.DeletePost(context.Background(), "42"); err != nil {
		t.Fatal(err)
	}
}

func TestCommonMessageUnsupportedBoundaries(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "-1001", false)
	text := "hello"
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "-1001", Text: &text, MediaIDs: []string{"file-1"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("media message error=%v", err)
	}
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "-1001", Text: &text, RecipientIDs: []string{"user-1"}}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("recipient message error=%v", err)
	}
	if _, err := client.GetMessage(context.Background(), "42"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message error=%v", err)
	}
	if _, err := client.PublishStatus(context.Background(), "42"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("publish status error=%v", err)
	}
}
