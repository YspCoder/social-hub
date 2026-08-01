package douyin

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookVerifyDecodeAndChallenge(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server)
	body := []byte(`{"event":"authorize","from_user_id":"open-id-1","client_key":"client-key","content":{"scopes":["user_info"]}}`)
	digest := sha1.Sum(append([]byte("client-secret"), body...))
	request := httptest.NewRequest(http.MethodPost, "/callback", nil)
	request.Header.Set("X-Douyin-Signature", hex.EncodeToString(digest[:]))
	request.Header.Set("Msg-Id", "message-1")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].ID != "message-1" || events[0].Type != "authorize" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	challenge, err := client.ChallengeResponse([]byte(`{"event":"verify_webhook","content":{"challenge":12345}}`))
	if err != nil || string(challenge) != `{"challenge":12345}` {
		t.Fatalf("challenge=%s err=%v", challenge, err)
	}
}
