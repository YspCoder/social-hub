package viber

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestWebhookVerificationAndMessageDecode(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	body := []byte(`{"event":"message","timestamp":1785672000000,"message_token":4912661846655238145,"sender":{"id":"subscriber-1","name":"Ada","avatar":"https://cdn.example/ada.jpg","country":"GB","language":"en","api_version":1},"message":{"type":"picture","text":"caption","media":"https://cdn.example/photo.jpg","tracking_data":"campaign"}}`)
	request := httptest.NewRequest(http.MethodPost, "https://bot.example/webhook", nil)
	mac := hmac.New(sha256.New, []byte("bot-token"))
	_, _ = mac.Write(body)
	request.Header.Set("X-Viber-Content-Signature", hex.EncodeToString(mac.Sum(nil)))
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%#v error=%v", events, err)
	}
	event := events[0]
	if event.ID != "message:4912661846655238145" || event.Type != "viber.message.picture" || event.Platform != "viber" || event.AccountID != "main" {
		t.Fatalf("event=%#v", event)
	}
	payload, ok := event.Payload.(WebhookEvent)
	if !ok || payload.Sender == nil || payload.Sender.ID != "subscriber-1" || payload.Message == nil || payload.Message.Type != "picture" ||
		payload.NormalizedMessage == nil || payload.NormalizedMessage.Direction != socialhub.DirectionInbound || len(payload.NormalizedMessage.Media) != 1 ||
		payload.Timestamp != time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC) {
		t.Fatalf("payload=%#v", payload)
	}

	request.Header.Set("X-Viber-Content-Signature", hex.EncodeToString(make([]byte, sha256.Size)))
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}
	request.Method = http.MethodGet
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("wrong method=%v", err)
	}
}

func TestWebhookDecodeOtherEventsAndValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	events, err := client.Decode(context.Background(), nil, []byte(`{"event":"failed","timestamp":1,"message_token":42,"user_id":"subscriber-1","desc":"receiver offline"}`))
	if err != nil || len(events) != 1 || events[0].Type != "viber.failed" {
		t.Fatalf("failed event=%#v error=%v", events, err)
	}
	payload := events[0].Payload.(WebhookEvent)
	if payload.UserID != "subscriber-1" || payload.Description != "receiver offline" || string(payload.Raw) == "" {
		t.Fatalf("payload=%#v", payload)
	}
	invalidBodies := [][]byte{
		nil,
		[]byte(`{`),
		[]byte(`{"timestamp":1,"message_token":42}`),
		[]byte(`{"event":"message","timestamp":1,"message_token":42,"message":{"type":"text","text":"hello"}}`),
		[]byte(`{"event":"message","timestamp":1,"message_token":42,"sender":{"id":"bad id","name":"Ada"},"message":{"type":"text","text":"hello"}}`),
	}
	for index, body := range invalidBodies {
		if _, err := client.Decode(context.Background(), nil, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid body %d error=%v", index, err)
		}
	}
	if _, err := client.SetWebhook(context.Background(), SetWebhookRequest{URL: "http://bot.example/webhook"}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("insecure webhook=%v", err)
	}
	if _, err := client.SetWebhook(context.Background(), SetWebhookRequest{URL: "https://bot.example/webhook", EventTypes: []WebhookEventType{"unknown"}}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("unknown event type=%v", err)
	}
}
