package line

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func webhookRequest(body []byte, secret string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/webhooks/line", nil)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	request.Header.Set("X-Line-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	return request
}

func TestWebhookVerifyAndDecode(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true)
	body := []byte(`{"destination":"` + testBotID + `","events":[` +
		`{"type":"message","mode":"active","timestamp":1722513600123,"webhookEventId":"event-1","source":{"type":"user","userId":"` + testUserID + `"},"replyToken":"reply-token","deliveryContext":{"isRedelivery":true},"message":{"id":"message-1","type":"text","text":"hello","quoteToken":"quote-token"}},` +
		`{"type":"postback","mode":"active","timestamp":1722513601123,"webhookEventId":"event-2","source":{"type":"user","userId":"` + testUserID + `"},"postback":{"data":"action=buy","params":{"date":"2026-08-01"}}},` +
		`{"type":"follow","mode":"active","timestamp":1722513602123,"webhookEventId":"event-3","source":{"type":"user","userId":"` + testUserID + `"}}]}`)
	request := webhookRequest(body, "channel-secret")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 3 {
		t.Fatalf("events=%#v error=%v", events, err)
	}
	if events[0].ID != "event-1" || events[0].Type != "line.message.text" || events[0].Platform != "line" || events[0].AccountID != "main" {
		t.Fatalf("message event=%#v", events[0])
	}
	messagePayload, ok := events[0].Payload.(WebhookEvent)
	if !ok || messagePayload.Message == nil || messagePayload.Message.Text != "hello" || messagePayload.Source == nil || messagePayload.Source.UserID != testUserID || !messagePayload.IsRedelivery || len(messagePayload.Raw) == 0 || len(messagePayload.Message.Raw) == 0 {
		t.Fatalf("message payload=%#v", events[0].Payload)
	}
	wantTimestamp := time.UnixMilli(1722513600123).UTC()
	if messagePayload.Timestamp == nil || !messagePayload.Timestamp.Equal(wantTimestamp) {
		t.Fatalf("timestamp=%v", messagePayload.Timestamp)
	}
	postbackPayload := events[1].Payload.(WebhookEvent)
	if events[1].Type != "line.postback" || postbackPayload.Postback == nil || postbackPayload.Postback.Data != "action=buy" || len(postbackPayload.Postback.Params) == 0 {
		t.Fatalf("postback=%#v", events[1])
	}
	if events[2].Type != "line.follow" {
		t.Fatalf("follow=%#v", events[2])
	}
}

func TestWebhookValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, true)
	_, tokenOnly := newTestAdapter(t, server, false)
	validBody := []byte(`{"destination":"` + testBotID + `","events":[]}`)

	if err := tokenOnly.Verify(context.Background(), webhookRequest(validBody, "channel-secret"), validBody); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("verify without secret=%v", err)
	}
	if err := client.Verify(context.Background(), nil, validBody); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("verify nil request=%v", err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/webhooks/line", nil)
	if err := client.Verify(context.Background(), getRequest, validBody); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("verify GET=%v", err)
	}
	badSignature := webhookRequest(validBody, "wrong-secret")
	if err := client.Verify(context.Background(), badSignature, validBody); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong signature=%v", err)
	}
	malformedSignature := httptest.NewRequest(http.MethodPost, "/webhooks/line", nil)
	malformedSignature.Header.Set("X-Line-Signature", "not-base64")
	if err := client.Verify(context.Background(), malformedSignature, validBody); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("malformed signature=%v", err)
	}

	invalidBodies := [][]byte{
		nil,
		[]byte("{"),
		[]byte(`{"destination":"bad","events":[]}`),
		[]byte(`{"destination":"` + testUserID + `","events":[]}`),
		[]byte(`{"destination":"` + testBotID + `","events":[null]}`),
		[]byte(`{"destination":"` + testBotID + `","events":[{"type":"message","timestamp":1,"webhookEventId":"event-1"}]}`),
		[]byte(`{"destination":"` + testBotID + `","events":[{"type":"message","timestamp":1,"webhookEventId":"event-1","message":{"id":"bad id","type":"text"}}]}`),
		[]byte(`{"destination":"` + testBotID + `","events":[{"type":"postback","timestamp":1,"webhookEventId":"event-1"}]}`),
		[]byte(`{"destination":"` + testBotID + `","events":[{"type":"postback","timestamp":1,"webhookEventId":"event-1","postback":{"data":" "}}]}`),
		[]byte(`{"destination":"` + testBotID + `","events":[{"type":"follow","timestamp":-1,"webhookEventId":"event-1"}]}`),
	}
	for index, body := range invalidBodies {
		if _, err := client.Decode(context.Background(), nil, body); err == nil {
			t.Fatalf("decode validation %d accepted: %q", index, strings.TrimSpace(string(body)))
		}
	}
}
