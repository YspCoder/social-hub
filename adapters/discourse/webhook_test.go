package discourse

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	_, client := newTestClient(t, server, false, true)
	body := []byte(`{"post":{"id":10,"topic_id":9}}`)
	request := signedWebhookRequest(body, "event-1", "post", "post_created", "webhook-secret")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].ID != "event-1" || events[0].Type != "discourse.post_created" || events[0].Platform != "discourse" || events[0].AccountID != "forum" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(WebhookPayload)
	if !ok || payload.EventID != "event-1" || payload.EventType != "post" || payload.Event != "post_created" || string(payload.Raw) != string(body) {
		t.Fatalf("payload=%#v", events[0].Payload)
	}
}

func TestWebhookValidationFailures(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, true)
	body := []byte(`{"post":{"id":10}}`)
	badSignature := signedWebhookRequest(body, "event-1", "post", "post_created", "wrong")
	if err := client.Verify(context.Background(), badSignature, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}
	request := signedWebhookRequest(body, "event-1", "post", "post_created", "webhook-secret")
	request.Method = http.MethodGet
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("method=%v", err)
	}
	if err := client.Verify(context.Background(), nil, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil request=%v", err)
	}
	request = signedWebhookRequest(body, "event-1", "post", "post_created", "webhook-secret")
	request.Header.Set("X-Discourse-Event-Signature", "sha256=not-hex")
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("signature format=%v", err)
	}
	request.Header.Set("X-Discourse-Event-Signature", "md5=abcd")
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("signature prefix=%v", err)
	}
	if err := client.Verify(context.Background(), signedWebhookRequest(nil, "event-1", "post", "post_created", "webhook-secret"), nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty body=%v", err)
	}
	oversized := make([]byte, maxWebhookBodyBytes+1)
	if err := client.Verify(context.Background(), signedWebhookRequest(oversized, "event-1", "post", "post_created", "webhook-secret"), oversized); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oversized body=%v", err)
	}

	decodeTests := []struct {
		name, id, eventType, event string
		body                       []byte
	}{
		{"event id", "bad id", "post", "post_created", body},
		{"event type", "event-1", "", "post_created", body},
		{"event", "event-1", "post", "", body},
		{"json", "event-1", "post", "post_created", []byte(`{`)},
	}
	for _, test := range decodeTests {
		t.Run(test.name, func(t *testing.T) {
			request := signedWebhookRequest(test.body, test.id, test.eventType, test.event, "webhook-secret")
			if _, err := client.Decode(context.Background(), request, test.body); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	_, noWebhook := newTestClient(t, server, true, false)
	if err := noWebhook.Verify(context.Background(), signedWebhookRequest(body, "event-1", "post", "post_created", "webhook-secret"), body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("missing secret=%v", err)
	}
	if !validWebhookValue("post_created", 20) || validWebhookValue("bad value", 20) || validWebhookValue(strings.Repeat("a", 21), 20) {
		t.Fatal("webhook header validation failed")
	}
}

func signedWebhookRequest(body []byte, eventID, eventType, event, secret string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "https://app.example/webhook", nil)
	request.Header.Set("X-Discourse-Event-Id", eventID)
	request.Header.Set("X-Discourse-Event-Type", eventType)
	request.Header.Set("X-Discourse-Event", event)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	request.Header.Set("X-Discourse-Event-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return request
}
