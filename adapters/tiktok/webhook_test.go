package tiktok

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestTikTokWebhookVerificationAndDecode(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, nil, true)
	body := []byte(`{"client_key":"client-key","event":"authorization.removed","create_time":1785542400,"user_openid":"open-id","content":"{\"reason\":1}"}`)
	timestamp := client.clock.Now().Unix()
	mac := hmac.New(sha256.New, []byte("client-secret"))
	_, _ = mac.Write([]byte(strconv.FormatInt(timestamp, 10) + "."))
	_, _ = mac.Write(body)
	request := httptest.NewRequest(http.MethodPost, "https://app.example/webhook", nil)
	request.Header.Set("TikTok-Signature", "t="+strconv.FormatInt(timestamp, 10)+",s="+hex.EncodeToString(mac.Sum(nil)))
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].Type != "tiktok.authorization.removed" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	stale := timestamp - int64((webhookTolerance+time.Second)/time.Second)
	request.Header.Set("TikTok-Signature", "t="+strconv.FormatInt(stale, 10)+",s="+hex.EncodeToString(mac.Sum(nil)))
	if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("stale error=%v", err)
	}
}
