package line

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

const testRetryKey = "123e4567-e89b-12d3-a456-426614174000"

func TestMessageContracts(t *testing.T) {
	pushCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer channel-token" || request.Header.Get("Idempotency-Key") != "" || request.Header.Get("Content-Type") != "application/json" {
			http.Error(writer, "bad headers", http.StatusUnauthorized)
			return
		}
		var body struct {
			To                     json.RawMessage  `json:"to"`
			ReplyToken             string           `json:"replyToken"`
			Messages               []map[string]any `json:"messages"`
			NotificationDisabled   bool             `json:"notificationDisabled"`
			CustomAggregationUnits []string         `json:"customAggregationUnits"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || len(body.Messages) == 0 {
			http.Error(writer, "bad body", http.StatusBadRequest)
			return
		}
		switch request.URL.Path {
		case "/v2/bot/message/push":
			if request.Header.Get("X-Line-Retry-Key") != testRetryKey {
				http.Error(writer, "bad retry key", http.StatusBadRequest)
				return
			}
			var to string
			_ = json.Unmarshal(body.To, &to)
			if to != testUserID {
				http.Error(writer, "bad target", http.StatusBadRequest)
				return
			}
			pushCalls++
			sent := make([]map[string]string, len(body.Messages))
			for index := range sent {
				sent[index] = map[string]string{"id": "push-" + string(rune('0'+pushCalls)) + "-" + string(rune('0'+index)), "quoteToken": "quote-result"}
			}
			writeTestJSON(t, writer, map[string]any{"sentMessages": sent})
		case "/v2/bot/message/reply":
			if request.Header.Get("X-Line-Retry-Key") != "" || body.ReplyToken != "reply-token" || len(body.Messages) != 2 || body.Messages[0]["type"] != "audio" || body.Messages[1]["type"] != "location" {
				http.Error(writer, "bad reply", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{"sentMessages": []any{map[string]string{"id": "reply-1"}, map[string]string{"id": "reply-2"}}})
		case "/v2/bot/message/multicast":
			if request.Header.Get("X-Line-Retry-Key") != testRetryKey || len(body.CustomAggregationUnits) != 1 || body.CustomAggregationUnits[0] != "campaign_1" {
				http.Error(writer, "bad multicast", http.StatusBadRequest)
				return
			}
			var recipients []string
			_ = json.Unmarshal(body.To, &recipients)
			if len(recipients) != 2 || recipients[0] != testUserID || recipients[1] != testUserID2 {
				http.Error(writer, "bad recipients", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{})
		case "/v2/bot/message/broadcast":
			if request.Header.Get("X-Line-Retry-Key") != testRetryKey || !body.NotificationDisabled || len(body.Messages) != 1 {
				http.Error(writer, "bad broadcast", http.StatusBadRequest)
				return
			}
			writeTestJSON(t, writer, map[string]any{})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true)

	text := "hello LINE"
	message, err := client.SendMessage(context.Background(), socialhub.SendMessageRequest{ConversationID: testUserID, Text: &text}, socialhub.WithIdempotencyKey(testRetryKey))
	if err != nil || message.ID != "push-1-0" || message.Text == nil || *message.Text != text || message.ConversationID != testUserID || len(message.RecipientIDs) != 1 || message.Direction != socialhub.DirectionOutbound || message.SentAt == nil || !message.SentAt.Equal(testNow) || len(message.Extensions) != 1 {
		t.Fatalf("common message=%#v error=%v", message, err)
	}
	push, err := client.Push(context.Background(), PushRequest{
		To: testUserID, NotificationDisabled: true, CustomAggregationUnits: []string{"campaign_1"},
		Messages: []MessageObject{
			TextMessage{Text: "text", QuoteToken: "quote-token"},
			StickerMessage{PackageID: "446", StickerID: "1988", QuoteToken: "quote-token"},
			ImageMessage{OriginalContentURL: "https://cdn.test/image.jpg", PreviewImageURL: "https://cdn.test/preview.jpg"},
			VideoMessage{OriginalContentURL: "https://cdn.test/video.mp4", PreviewImageURL: "https://cdn.test/video.jpg", TrackingID: "track-1"},
			LocationMessage{Title: "Office", Address: "Tokyo", Latitude: 35.68, Longitude: 139.76},
		},
	}, socialhub.WithIdempotencyKey(testRetryKey))
	if err != nil || len(push.SentMessages) != 5 || push.SentMessages[0].ID != "push-2-0" || push.SentMessages[0].QuoteToken != "quote-result" {
		t.Fatalf("push=%#v error=%v", push, err)
	}
	reply, err := client.Reply(context.Background(), ReplyRequest{
		ReplyToken: "reply-token", Messages: []MessageObject{
			AudioMessage{OriginalContentURL: "https://cdn.test/audio.m4a", Duration: 2 * time.Second},
			LocationMessage{Title: "Station", Address: "Shibuya", Latitude: 35.65, Longitude: 139.70},
		},
	}, socialhub.WithIdempotencyKey(testRetryKey))
	if err != nil || len(reply.SentMessages) != 2 || reply.SentMessages[1].ID != "reply-2" {
		t.Fatalf("reply=%#v error=%v", reply, err)
	}
	if err := client.Multicast(context.Background(), MulticastRequest{
		To: []string{testUserID, testUserID2}, Messages: []MessageObject{TextMessage{Text: "many"}}, CustomAggregationUnits: []string{"campaign_1"},
	}, socialhub.WithIdempotencyKey(testRetryKey)); err != nil {
		t.Fatal(err)
	}
	if err := client.Broadcast(context.Background(), BroadcastRequest{
		Messages: []MessageObject{TextMessage{Text: "all"}}, NotificationDisabled: true,
	}, socialhub.WithIdempotencyKey(testRetryKey)); err != nil {
		t.Fatal(err)
	}
}

func TestCommonMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true)
	text, blank, reply := "hello", " ", "message-1"
	cases := []socialhub.SendMessageRequest{
		{}, {ConversationID: "bad", Text: &text}, {ConversationID: testUserID}, {ConversationID: testUserID, Text: &blank},
		{ConversationID: testUserID, Text: &text, RecipientIDs: []string{testUserID2}},
		{ConversationID: testUserID, Text: &text, MediaIDs: []string{"media"}},
		{ConversationID: testUserID, Text: &text, ReplyToID: &reply},
	}
	for index, input := range cases {
		if _, err := client.SendMessage(context.Background(), input); err == nil {
			t.Fatalf("common validation %d accepted", index)
		}
	}
	if _, err := client.GetMessage(context.Background(), "message-1"); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("get message=%v", err)
	}
}

func TestTypedMessageValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true)
	var nilText *TextMessage
	tooMany := make([]MessageObject, 6)
	for index := range tooMany {
		tooMany[index] = TextMessage{Text: "x"}
	}
	tooManyRecipients := make([]string, 501)
	for index := range tooManyRecipients {
		tooManyRecipients[index] = testUserID
	}
	invalid := []func() error{
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: "bad", Messages: []MessageObject{TextMessage{Text: "x"}}})
			return err
		},
		func() error { _, err := client.Push(context.Background(), PushRequest{To: testUserID}); return err },
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: tooMany})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{nilText}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{TextMessage{Text: strings.Repeat("x", 5001)}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{TextMessage{Text: "x", QuoteToken: "bad token"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{StickerMessage{}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{ImageMessage{OriginalContentURL: "http://cdn.test/a", PreviewImageURL: "https://cdn.test/b"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{VideoMessage{OriginalContentURL: "https://cdn.test/a", PreviewImageURL: "bad"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{VideoMessage{OriginalContentURL: "https://cdn.test/a", PreviewImageURL: "https://cdn.test/b", TrackingID: "bad#id"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testGroupID, Messages: []MessageObject{VideoMessage{OriginalContentURL: "https://cdn.test/a", PreviewImageURL: "https://cdn.test/b", TrackingID: "track-1"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{ImageMessage{OriginalContentURL: "https://cdn.test/" + strings.Repeat("x", 2000), PreviewImageURL: "https://cdn.test/b"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{AudioMessage{OriginalContentURL: "https://cdn.test/a"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{LocationMessage{Title: "x", Address: "y", Latitude: math.NaN()}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{LocationMessage{Title: "x", Address: "y", Latitude: 91}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{LocationMessage{Title: strings.Repeat("x", 101), Address: "y"}}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{TextMessage{Text: "x"}}, CustomAggregationUnits: []string{"one", "two"}})
			return err
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{TextMessage{Text: "x"}}, CustomAggregationUnits: []string{"bad-unit"}})
			return err
		},
		func() error {
			_, err := client.Reply(context.Background(), ReplyRequest{Messages: []MessageObject{TextMessage{Text: "x"}}})
			return err
		},
		func() error { return client.Multicast(context.Background(), MulticastRequest{}) },
		func() error {
			return client.Multicast(context.Background(), MulticastRequest{To: tooManyRecipients, Messages: []MessageObject{TextMessage{Text: "x"}}})
		},
		func() error {
			return client.Multicast(context.Background(), MulticastRequest{To: []string{testGroupID}, Messages: []MessageObject{TextMessage{Text: "x"}}})
		},
		func() error {
			return client.Multicast(context.Background(), MulticastRequest{To: []string{testUserID, testUserID}, Messages: []MessageObject{TextMessage{Text: "x"}}})
		},
		func() error {
			_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{TextMessage{Text: "x"}}}, socialhub.WithIdempotencyKey("bad"))
			return err
		},
	}
	for index, call := range invalid {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("validation %d=%v", index, err)
		}
	}
}

func TestMalformedSendResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeTestJSON(t, writer, map[string]any{"sentMessages": []any{}})
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, true)
	_, err := client.Push(context.Background(), PushRequest{To: testUserID, Messages: []MessageObject{TextMessage{Text: "x"}}})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError {
		t.Fatalf("malformed response=%#v", err)
	}
}
