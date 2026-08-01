package page

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestWebhookVerifyDecodeAndChallenge(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	body := []byte(`{"object":"page","entry":[{"id":"123","time":1785542400,"changes":[{"field":"feed","value":{"item":"post","post_id":"123_456"}}],"messaging":[{"sender":{"id":"9"}}]}]}`)
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, _ = mac.Write(body)
	request := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 2 || events[0].ID == "" || events[0].Type != "page.feed" {
		t.Fatalf("events = %#v, err = %v", events, err)
	}

	badRequest := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	badRequest.Header.Set("X-Hub-Signature-256", "sha256=00")
	if err := client.Verify(context.Background(), badRequest, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature error = %v", err)
	}
	challenge := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=challenge-value", nil)
	status, response, err := client.HandleChallenge(context.Background(), challenge)
	if err != nil || status != http.StatusOK || string(response) != "challenge-value" {
		t.Fatalf("challenge status=%d body=%q err=%v", status, response, err)
	}
}
