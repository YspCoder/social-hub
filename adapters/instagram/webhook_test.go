package instagram

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
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, nil, true)
	body := []byte(`{"object":"instagram","entry":[{"id":"178","time":1785542400,"changes":[{"field":"comments","value":{"id":"comment-1"}}]}]}`)
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, _ = mac.Write(body)
	request := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].Type != "instagram.comments" || events[0].ID == "" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	bad := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	bad.Header.Set("X-Hub-Signature-256", "sha256=00")
	if err := client.Verify(context.Background(), bad, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature error=%v", err)
	}
	challenge := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=ok", nil)
	status, response, err := client.HandleChallenge(context.Background(), challenge)
	if err != nil || status != http.StatusOK || string(response) != "ok" {
		t.Fatalf("status=%d body=%s err=%v", status, response, err)
	}
}
