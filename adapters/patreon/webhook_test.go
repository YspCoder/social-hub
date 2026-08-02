package patreon

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
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
	_, client := newTestClient(t, server, false, true, nil)
	body := []byte(`{"data":{"type":"member","id":"member-1","attributes":{"patron_status":"active_patron"}}}`)
	request := signedWebhookRequest(t, body, "members:update", "webhook-secret")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || len(events[0].ID) != 64 || events[0].Type != "members:update" || events[0].Platform != "patreon" || events[0].AccountID != "creator" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(WebhookPayload)
	if !ok || payload.Trigger != "members:update" || payload.DataType != "member" || payload.DataID != "member-1" || string(payload.Raw) != string(body) {
		t.Fatalf("payload=%#v", events[0].Payload)
	}
	again, err := client.Decode(context.Background(), request, body)
	if err != nil || again[0].ID != events[0].ID {
		t.Fatalf("deterministic event=%#v err=%v", again, err)
	}
}

func TestWebhookValidationFailures(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, false, true, nil)
	body := []byte(`{"data":{"type":"post","id":"200"}}`)
	badSignature := signedWebhookRequest(t, body, "posts:publish", "wrong-secret")
	if err := client.Verify(context.Background(), badSignature, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}
	request := signedWebhookRequest(t, body, "posts:publish", "webhook-secret")
	request.Method = http.MethodGet
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("method=%v", err)
	}
	if err := client.Verify(context.Background(), nil, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil request=%v", err)
	}
	request = signedWebhookRequest(t, body, "posts:publish", "webhook-secret")
	request.Header.Set("X-Patreon-Signature", "not-hex")
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("signature format=%v", err)
	}
	if err := client.Verify(context.Background(), signedWebhookRequest(t, nil, "posts:publish", "webhook-secret"), nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty body=%v", err)
	}
	oversized := make([]byte, maxWebhookBodyBytes+1)
	if err := client.Verify(context.Background(), signedWebhookRequest(t, oversized, "posts:publish", "webhook-secret"), oversized); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oversized body=%v", err)
	}

	decodeTests := []struct {
		name    string
		body    []byte
		trigger string
	}{
		{"trigger", body, "BAD TRIGGER"},
		{"json", []byte(`not-json`), "posts:update"},
		{"data", []byte(`{"data":{}}`), "posts:delete"},
	}
	for _, test := range decodeTests {
		t.Run(test.name, func(t *testing.T) {
			request := signedWebhookRequest(t, test.body, test.trigger, "webhook-secret")
			if _, err := client.Decode(context.Background(), request, test.body); !errors.Is(err, socialhub.ErrInvalidArgument) {
				t.Fatalf("error=%v", err)
			}
		})
	}

	_, noWebhook := newTestClient(t, server, true, false, []string{"identity"})
	if err := noWebhook.Verify(context.Background(), signedWebhookRequest(t, body, "posts:publish", "webhook-secret"), body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("missing secret=%v", err)
	}
	if !validWebhookTrigger("posts:new_type-1.0") || validWebhookTrigger("Posts:publish") || validWebhookTrigger(strings.Repeat("a", 129)) {
		t.Fatal("trigger validation contract failed")
	}
}

func signedWebhookRequest(t *testing.T, body []byte, trigger, secret string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "https://app.example/webhook", nil)
	request.Header.Set("X-Patreon-Event", trigger)
	mac := hmac.New(md5.New, []byte(secret))
	_, _ = mac.Write(body)
	request.Header.Set("X-Patreon-Signature", hex.EncodeToString(mac.Sum(nil)))
	return request
}
