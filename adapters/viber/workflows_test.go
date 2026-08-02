package viber

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestMessageAccountAndWebhookManagementWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("X-Viber-Auth-Token") != "bot-token" || request.Header.Get("Authorization") != "" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/pa/send_message":
			var receiver, messageType string
			var sender Sender
			_ = json.Unmarshal(body["receiver"], &receiver)
			_ = json.Unmarshal(body["type"], &messageType)
			_ = json.Unmarshal(body["sender"], &sender)
			if receiver != "subscriber-1" || sender.Name != "Social Hub" || sender.Avatar == "" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			if messageType == "text" {
				var text string
				_ = json.Unmarshal(body["text"], &text)
				if text == "rate" {
					writeTestJSON(t, writer, map[string]any{"status": 12, "status_message": "tooManyRequests"})
					return
				}
				if text == "malformed" {
					writeTestJSON(t, writer, map[string]any{"status": 0, "message_token": 1003})
					return
				}
				if text == "bad-billing" {
					writeTestJSON(t, writer, map[string]any{"status": 0, "status_message": "ok", "message_token": 1004, "billing_status": 6})
					return
				}
				writeTestJSON(t, writer, map[string]any{"status": 0, "status_message": "ok", "message_token": 1001, "chat_hostname": "SN-CHAT", "billing_status": 1})
				return
			}
			if messageType != "picture" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"status": 0, "status_message": "ok", "message_token": 1002})
		case "/pa/broadcast_message":
			var receivers []string
			_ = json.Unmarshal(body["broadcast_list"], &receivers)
			if !reflect.DeepEqual(receivers, []string{"subscriber-1", "subscriber-2"}) {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"status": 0, "status_message": "ok", "message_token": 2001,
				"failed_list": []map[string]any{{"receiver": "subscriber-2", "status": 6, "status_message": "Not subscribed"}},
			})
		case "/pa/get_account_info":
			writeTestJSON(t, writer, map[string]any{
				"status": 0, "status_message": "ok", "id": "pa:123", "name": "Social Hub Bot", "uri": "socialhub",
				"icon": "https://cdn.example/icon.jpg", "country": "SG", "webhook": "https://bot.example/webhook",
				"event_types": []string{"message"}, "subscribers_count": 42,
			})
		case "/pa/get_user_details":
			var id string
			_ = json.Unmarshal(body["id"], &id)
			if id == "http-error" {
				writer.WriteHeader(http.StatusUnauthorized)
				writeTestJSON(t, writer, map[string]any{"status": 2, "status_message": "invalidAuthToken"})
				return
			}
			writeTestJSON(t, writer, map[string]any{
				"status": 0, "status_message": "ok", "message_token": 3001,
				"user": map[string]any{"id": id, "name": "Ada", "avatar": "https://cdn.example/ada.jpg", "country": "GB", "language": "en", "api_version": 1},
			})
		case "/pa/get_online":
			writeTestJSON(t, writer, map[string]any{
				"status": 0, "status_message": "ok",
				"users": []map[string]any{{"id": "subscriber-1", "online_status": 1, "online_status_message": "offline", "last_online": 1700000000000}},
			})
		case "/pa/set_webhook":
			var webhookURL string
			_ = json.Unmarshal(body["url"], &webhookURL)
			if webhookURL == "" {
				writeTestJSON(t, writer, map[string]any{"status": 0, "status_message": "ok"})
				return
			}
			if webhookURL != "https://bot.example/webhook" {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"status": 0, "status_message": "ok", "event_types": []string{"delivered", "message"}})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	text := "hello"
	common, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{
		ConversationID: "subscriber-1", RecipientIDs: []string{"subscriber-1"}, Text: &text,
	})
	if err != nil || common.ID != "1001" || common.ConversationID != "subscriber-1" || common.Text == nil || *common.Text != text ||
		common.Direction != socialhub.DirectionOutbound || common.SentAt == nil || !common.SentAt.Equal(testNow) {
		t.Fatalf("common message=%#v error=%v", common, err)
	}
	typed, err := client.Send(context.Background(), SendRequest{
		Receiver: "subscriber-1", TrackingData: "campaign-1", MinAPIVersion: 1,
		Message: PictureMessage{Text: "photo", MediaURL: "https://cdn.example/photo.jpeg", ThumbnailURL: "https://cdn.example/thumb.jpg"},
	})
	if err != nil || typed.MessageToken.String() != "1002" {
		t.Fatalf("typed send=%#v error=%v", typed, err)
	}
	broadcast, err := client.Broadcast(context.Background(), BroadcastRequest{
		Receivers: []string{"subscriber-1", "subscriber-2"}, Message: TextMessage{Text: "announcement"},
	})
	if err != nil || broadcast.MessageToken.String() != "2001" || len(broadcast.FailedList) != 1 || broadcast.FailedList[0].Status != 6 {
		t.Fatalf("broadcast=%#v error=%v", broadcast, err)
	}

	account, err := client.GetAccountInfo(context.Background())
	if err != nil || account.ID != "pa:123" || account.SubscribersCount != 42 {
		t.Fatalf("account=%#v error=%v", account, err)
	}
	user, err := client.GetUserDetails(context.Background(), "subscriber-1")
	if err != nil || user.ID != "subscriber-1" || user.Name != "Ada" {
		t.Fatalf("user=%#v error=%v", user, err)
	}
	commonUser, err := client.GetUser(context.Background(), "subscriber-1")
	if err != nil || commonUser.ID != "subscriber-1" || commonUser.DisplayName == nil || *commonUser.DisplayName != "Ada" {
		t.Fatalf("common user=%#v error=%v", commonUser, err)
	}
	bot, err := client.GetUser(context.Background(), "me")
	if err != nil || bot.ID != "pa:123" || bot.Username == nil || *bot.Username != "socialhub" || bot.ProfileURL == nil {
		t.Fatalf("bot=%#v error=%v", bot, err)
	}
	online, err := client.GetOnline(context.Background(), []string{"subscriber-1", "subscriber-2"})
	if err != nil || len(online) != 1 || online[0].State != Offline || online[0].LastOnlineMillis != 1700000000000 {
		t.Fatalf("online=%#v error=%v", online, err)
	}
	sendName, sendPhoto := false, true
	webhook, err := client.SetWebhook(context.Background(), SetWebhookRequest{
		URL: "https://bot.example/webhook", EventTypes: []WebhookEventType{WebhookDelivered, WebhookMessage},
		SendName: &sendName, SendPhoto: &sendPhoto,
	})
	if err != nil || len(webhook.EventTypes) != 2 {
		t.Fatalf("webhook=%#v error=%v", webhook, err)
	}
	if err := client.RemoveWebhook(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Send(context.Background(), SendRequest{Receiver: "subscriber-1", Message: TextMessage{Text: "rate"}}); !errors.Is(err, socialhub.ErrRateLimited) {
		t.Fatalf("embedded rate error=%v", err)
	}
	if _, err := client.Send(context.Background(), SendRequest{Receiver: "subscriber-1", Message: TextMessage{Text: "malformed"}}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("missing status message=%v", err)
	}
	if _, err := client.Send(context.Background(), SendRequest{Receiver: "subscriber-1", Message: TextMessage{Text: "bad-billing"}}); errorCode(err) != socialhub.CodePlatformError {
		t.Fatalf("invalid billing status=%v", err)
	}
	if _, err := client.GetUserDetails(context.Background(), "http-error"); !errors.Is(err, socialhub.ErrUnauthenticated) {
		t.Fatalf("HTTP auth error=%v", err)
	}
}

func TestUnsupportedCommonOperations(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	if _, err := client.GetPost(context.Background(), "post"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get post=%v", err)
	}
	if _, err := client.ListPosts(context.Background(), socialhub.ListPostsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list posts=%v", err)
	}
	if _, err := client.ListComments(context.Background(), socialhub.ListCommentsRequest{}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("list comments=%v", err)
	}
	if _, err := client.GetMessage(context.Background(), "message"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message=%v", err)
	}
	media := []string{"https://cdn.example/a.jpg"}
	text := "hello"
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "subscriber-1", Text: &text, MediaIDs: media}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("common media=%v", err)
	}
	reply := "123"
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "subscriber-1", Text: &text, ReplyToID: &reply}); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("common reply=%v", err)
	}
	if _, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: "subscriber-1", RecipientIDs: []string{"other"}, Text: &text}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("common recipient=%v", err)
	}
	if _, err := client.Send(context.Background(), SendRequest{Receiver: "subscriber-1", TrackingData: strings.Repeat("x", 4097), Message: TextMessage{Text: "x"}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("tracking limit=%v", err)
	}
	if _, err := client.Send(context.Background(), SendRequest{Receiver: "subscriber-1", MinAPIVersion: 100, Message: TextMessage{Text: "x"}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("API version=%v", err)
	}
	largeRecipients := make([]string, 300)
	for index := range largeRecipients {
		largeRecipients[index] = strings.Repeat("x", 120) + strconv.Itoa(index)
	}
	if _, err := client.Broadcast(context.Background(), BroadcastRequest{Receivers: largeRecipients, Message: TextMessage{Text: "x"}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("request size=%v", err)
	}
}
