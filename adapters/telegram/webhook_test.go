package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestWebhookVerifyAndDecode(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "", true)
	body := []byte(`{"update_id":0,"message":{"message_id":7,"from":{"id":9,"is_bot":false,"first_name":"User"},"date":1785542400,"chat":{"id":9,"type":"private"},"text":"hello"}}`)
	request := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	request.Header.Set("X-Telegram-Bot-Api-Secret-Token", "webhook_secret-1")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].ID != "0" || events[0].Type != "message" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(UpdatePayload)
	if !ok || payload.Update.Message == nil || payload.Update.Message.Text != "hello" || len(payload.Raw) == 0 {
		t.Fatalf("payload=%#v", events[0].Payload)
	}
	bad := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	bad.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	if err := client.Verify(context.Background(), bad, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad secret error=%v", err)
	}
}

func TestWebhookPreservesUnknownCurrentFields(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, "", true)
	body := []byte(`{"update_id":8,"subscription":{"status":"active"}}`)
	events, err := client.Decode(context.Background(), nil, body)
	if err != nil || len(events) != 1 || events[0].Type != "subscription" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(UpdatePayload)
	if !ok || !strings.Contains(string(payload.Raw), `"subscription"`) {
		t.Fatalf("payload=%#v", events[0].Payload)
	}
}
