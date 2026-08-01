package qq

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func TestWebhookVerifyDecodeAndStableEventID(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	body := []byte(`{"id":"event-1","op":0,"s":42,"t":"GROUP_AT_MESSAGE_CREATE","d":{"id":"message-1"}}`)
	timestamp := "1785542400"
	request := signedWebhookRequest(t, "app-secret", timestamp, body)
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].ID != "event-1" || events[0].Type != "qq.group_at_message_create" || events[0].Platform != "qq" || events[0].AccountID != "main" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(WebhookEvent)
	if !ok || payload.Sequence == nil || *payload.Sequence != 42 || !bytes.Equal(payload.Data, []byte(`{"id":"message-1"}`)) || !bytes.Equal(payload.Raw, body) {
		t.Fatalf("payload=%#v", events[0].Payload)
	}

	withoutID := []byte(`{"op":0,"t":"READY","d":{"version":1}}`)
	first, err := client.Decode(context.Background(), signedWebhookRequest(t, "app-secret", timestamp, withoutID), withoutID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Decode(context.Background(), signedWebhookRequest(t, "app-secret", timestamp, withoutID), withoutID)
	if err != nil || len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID || len(first[0].ID) != 64 {
		t.Fatalf("stable events first=%#v second=%#v err=%v", first, second, err)
	}
}

func TestWebhookValidationResponse(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	body := []byte(`{"op":13,"d":{"plain_token":"plain-token","event_ts":"1785542400"}}`)
	response, err := client.ValidationResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		PlainToken string `json:"plain_token"`
		Signature  string `json:"signature"`
	}
	if err := json.Unmarshal(response, &payload); err != nil || payload.PlainToken != "plain-token" {
		t.Fatalf("response=%s err=%v", response, err)
	}
	signature, err := hex.DecodeString(payload.Signature)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, _, err := webhookKeys("app-secret")
	if err != nil || !ed25519.Verify(publicKey, []byte("1785542400plain-token"), signature) {
		t.Fatalf("validation signature=%q err=%v", payload.Signature, err)
	}
}

func TestWebhookRejectsTamperingAndInvalidPayloads(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	_, noWebhook := newTestClient(t, server, false)
	body := []byte(`{"op":0,"t":"READY","d":{}}`)
	request := signedWebhookRequest(t, "app-secret", "1785542400", body)

	if err := client.Verify(context.Background(), request, append(body, 'x')); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("tampered body=%v", err)
	}
	badSignature := request.Clone(context.Background())
	badSignature.Header.Set(webhookSignatureHeader, "not-hex")
	if err := client.Verify(context.Background(), badSignature, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}
	missingTimestamp := request.Clone(context.Background())
	missingTimestamp.Header.Del(webhookTimestampHeader)
	if err := client.Verify(context.Background(), missingTimestamp, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("missing timestamp=%v", err)
	}
	if err := noWebhook.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured verify=%v", err)
	}
	if _, err := noWebhook.ValidationResponse(body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured validation=%v", err)
	}

	get := httptest.NewRequest(http.MethodGet, "/callback", nil)
	if err := client.Verify(context.Background(), get, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("GET verify=%v", err)
	}
	invalidDispatches := [][]byte{
		[]byte(`{`),
		[]byte(`{"op":13,"d":{}}`),
		[]byte(`{"op":0,"t":" bad ","d":{}}`),
		[]byte(`{"op":0,"t":"READY","d":null}`),
		[]byte(`{"id":"bad/id","op":0,"t":"READY","d":{}}`),
	}
	for index, invalid := range invalidDispatches {
		if _, err := client.Decode(context.Background(), request, invalid); err == nil {
			t.Fatalf("invalid dispatch %d unexpectedly succeeded", index)
		}
	}
	invalidValidations := [][]byte{
		[]byte(`{`),
		[]byte(`{"op":0,"d":{"plain_token":"token","event_ts":"time"}}`),
		[]byte(`{"op":13,"d":{"plain_token":"","event_ts":"time"}}`),
	}
	for index, invalid := range invalidValidations {
		if _, err := client.ValidationResponse(invalid); err == nil {
			t.Fatalf("invalid validation %d unexpectedly succeeded", index)
		}
	}
	if _, _, err := webhookKeys(""); err == nil {
		t.Fatal("empty webhook secret accepted")
	}
	oversized := []byte(strings.Repeat("x", maxWebhookBodyBytes+1))
	if err := client.Verify(context.Background(), request, oversized); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("oversized verify=%v", err)
	}
}

func signedWebhookRequest(t *testing.T, secret, timestamp string, body []byte) *http.Request {
	t.Helper()
	_, privateKey, err := webhookKeys(secret)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(privateKey, append([]byte(timestamp), body...))
	request := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	request.Header.Set(webhookTimestampHeader, timestamp)
	request.Header.Set(webhookSignatureHeader, hex.EncodeToString(signature))
	return request
}
