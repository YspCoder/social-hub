package zalo

import (
	"context"
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
	_, client := newTestClient(t, server, false, true)
	body := []byte(`{"app_id":"360846524940903967","sender":{"id":"2512523625412515"},"user_id_by_app":"552177279717587730","recipient":{"id":"388613280878808645"},"event_name":"user_send_image","message":{"text":"photo","msg_id":"96d3cdf3af150460909","quote_msg_id":"prior-message","attachments":[{"type":"image","payload":{"url":"https://cdn.example/photo.jpg","thumbnail":"https://cdn.example/thumb.jpg"}}]},"timestamp":"1785672000000"}`)
	request := httptest.NewRequest(http.MethodPost, "https://app.example/webhook", nil)
	hash := sha256.New()
	_, _ = hash.Write([]byte("360846524940903967"))
	_, _ = hash.Write(body)
	_, _ = hash.Write([]byte("1785672000000"))
	_, _ = hash.Write([]byte("oa-secret"))
	request.Header.Set("X-ZEvent-Signature", "mac="+hex.EncodeToString(hash.Sum(nil)))
	request.Header.Set("num_retry", "2")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].ID != "user_send_image:96d3cdf3af150460909" || events[0].Type != "zalo.user_send_image" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(WebhookEvent)
	if !ok || payload.RetryCount != 2 || payload.Message == nil || payload.Message.ID != "96d3cdf3af150460909" ||
		payload.NormalizedMessage == nil || payload.NormalizedMessage.Direction != socialhub.DirectionInbound ||
		payload.NormalizedMessage.ConversationID != "2512523625412515" || payload.NormalizedMessage.ReplyToID == nil ||
		len(payload.NormalizedMessage.Media) != 1 || payload.NormalizedMessage.Media[0].Type != socialhub.MediaTypeImage ||
		payload.Timestamp != time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC) {
		t.Fatalf("payload=%#v", payload)
	}

	request.Header.Set("X-ZEvent-Signature", hex.EncodeToString(make([]byte, sha256.Size)))
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}
	request.Method = http.MethodGet
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("wrong method=%v", err)
	}
}

func TestWebhookOtherEventsAndValidation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, true)
	body := []byte(`{"app_id":"360846524940903967","sender":{"id":"388613280878808645"},"recipient":{"id":"2512523625412515"},"event_name":"follow","timestamp":"1785672000000"}`)
	events, err := client.Decode(context.Background(), nil, body)
	if err != nil || len(events) != 1 || events[0].ID == "" || events[0].Type != "zalo.follow" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload := events[0].Payload.(WebhookEvent)
	if payload.Message != nil || payload.NormalizedMessage != nil || len(payload.Raw) == 0 {
		t.Fatalf("payload=%#v", payload)
	}
	invalidBodies := [][]byte{
		nil,
		[]byte(`{`),
		[]byte(`{"app_id":"wrong","sender":{"id":"1"},"recipient":{"id":"388613280878808645"},"event_name":"follow","timestamp":"1"}`),
		[]byte(`{"app_id":"360846524940903967","sender":{"id":"1"},"recipient":{"id":"2"},"event_name":"follow","timestamp":"1"}`),
		[]byte(`{"app_id":"360846524940903967","sender":{"id":"1"},"recipient":{"id":"388613280878808645"},"event_name":"user_send_text","message":{"text":"missing id"},"timestamp":"1"}`),
	}
	for index, invalid := range invalidBodies {
		if _, err := client.Decode(context.Background(), nil, invalid); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid body %d error=%v", index, err)
		}
	}
	badRetry := httptest.NewRequest(http.MethodPost, "https://app.example/webhook", nil)
	badRetry.Header.Set("num_retry", "bad")
	if _, err := client.Decode(context.Background(), badRetry, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("bad retry=%v", err)
	}
}
