package messenger

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

func signedWebhookRequest(body []byte, secret string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "https://app.example/webhook", nil)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return request
}

func TestWebhookVerificationChallengeAndMessageDecode(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	body := []byte(`{"object":"page","entry":[{"id":"123456789","time":1785672000,"messaging":[{"sender":{"id":"111222333"},"recipient":{"id":"123456789"},"timestamp":1785672000000,"message":{"mid":"mid.inbound","text":"hello","quick_reply":{"payload":"ORDER_STATUS"},"reply_to":{"mid":"mid.parent"},"attachments":[{"type":"image","payload":{"url":"https://cdn.example/photo.jpg"}}]}}]}]}`)
	request := signedWebhookRequest(body, "app-secret")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 1 || events[0].ID != "mid.inbound" || events[0].Type != "facebook.messenger.message" ||
		events[0].Platform != "facebook" || events[0].AccountID != "main" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	payload, ok := events[0].Payload.(WebhookEvent)
	wantTime := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	if !ok || payload.PageID != "123456789" || payload.Message == nil || payload.Message.QuickReply == nil ||
		payload.Message.QuickReply.Payload != "ORDER_STATUS" || payload.NormalizedMessage == nil ||
		payload.NormalizedMessage.Direction != socialhub.DirectionInbound || payload.NormalizedMessage.ConversationID != "111222333" ||
		payload.NormalizedMessage.SenderID == nil || *payload.NormalizedMessage.SenderID != "111222333" ||
		payload.NormalizedMessage.ReplyToID == nil || *payload.NormalizedMessage.ReplyToID != "mid.parent" ||
		len(payload.NormalizedMessage.Media) != 1 || payload.NormalizedMessage.Media[0].Type != socialhub.MediaTypeImage ||
		!payload.Timestamp.Equal(wantTime) || !payload.EntryTime.Equal(wantTime) || len(payload.Raw) == 0 {
		t.Fatalf("payload=%#v", payload)
	}

	challenge := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=challenge-value", nil)
	status, response, err := client.HandleChallenge(context.Background(), challenge)
	if err != nil || status != http.StatusOK || string(response) != "challenge-value" {
		t.Fatalf("challenge status=%d body=%q err=%v", status, response, err)
	}
}

func TestWebhookReceiptsPostbacksReactionsAndGenericEvents(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	body := []byte(`{"object":"page","entry":[{"id":"123456789","time":1785672000,"messaging":[` +
		`{"sender":{"id":"111222333"},"recipient":{"id":"123456789"},"timestamp":1785672000001,"delivery":{"mids":["mid.one","mid.two"],"watermark":1785672000000}},` +
		`{"sender":{"id":"111222333"},"recipient":{"id":"123456789"},"timestamp":1785672000002,"read":{"watermark":1785672000001}},` +
		`{"sender":{"id":"111222333"},"recipient":{"id":"123456789"},"timestamp":1785672000003,"postback":{"mid":"mid.postback","title":"Start","payload":"GET_STARTED"}},` +
		`{"sender":{"id":"111222333"},"recipient":{"id":"123456789"},"timestamp":1785672000004,"reaction":{"mid":"mid.target","action":"react","reaction":"love","emoji":"heart"}},` +
		`{"sender":{"id":"111222333"},"recipient":{"id":"123456789"},"timestamp":1785672000005,"optin":{"ref":"campaign"}},` +
		`{"sender":{"id":"123456789"},"recipient":{"id":"111222333"},"timestamp":1785672000006,"message":{"mid":"mid.echo","text":"sent","is_echo":true}}]}]}`)
	events, err := client.Decode(context.Background(), nil, body)
	if err != nil || len(events) != 6 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	wantTypes := []string{
		"facebook.messenger.delivery", "facebook.messenger.read", "facebook.messenger.postback",
		"facebook.messenger.reaction", "facebook.messenger.event", "facebook.messenger.message",
	}
	seen := make(map[string]struct{}, len(events))
	for index, event := range events {
		if event.Type != wantTypes[index] || event.ID == "" {
			t.Fatalf("event %d=%#v", index, event)
		}
		if _, duplicate := seen[event.ID]; duplicate {
			t.Fatalf("duplicate event ID=%q", event.ID)
		}
		seen[event.ID] = struct{}{}
	}
	if payload := events[0].Payload.(WebhookEvent); payload.Delivery == nil || len(payload.Delivery.MessageIDs) != 2 {
		t.Fatalf("delivery payload=%#v", payload)
	}
	if payload := events[2].Payload.(WebhookEvent); payload.Postback == nil || payload.Postback.Payload != "GET_STARTED" {
		t.Fatalf("postback payload=%#v", payload)
	}
	if payload := events[3].Payload.(WebhookEvent); payload.Reaction == nil || payload.Reaction.Action != "react" {
		t.Fatalf("reaction payload=%#v", payload)
	}
	if payload := events[4].Payload.(WebhookEvent); payload.Message != nil || payload.NormalizedMessage != nil || len(payload.Raw) == 0 {
		t.Fatalf("generic payload=%#v", payload)
	}
	echo := events[5].Payload.(WebhookEvent).NormalizedMessage
	if echo == nil || echo.Direction != socialhub.DirectionOutbound || echo.ConversationID != "111222333" {
		t.Fatalf("echo=%#v", echo)
	}
	repeated, err := client.Decode(context.Background(), nil, body)
	if err != nil {
		t.Fatal(err)
	}
	for index := range events {
		if events[index].ID != repeated[index].ID {
			t.Fatalf("unstable ID %d: %q != %q", index, events[index].ID, repeated[index].ID)
		}
	}
}

func TestWebhookRejectsInvalidVerificationAndPayloads(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server, true)
	body := []byte(`{"object":"page","entry":[{"id":"123456789","time":1,"messaging":[]}]}`)
	verifyCases := []*http.Request{
		nil,
		httptest.NewRequest(http.MethodGet, "/webhook", nil),
	}
	for _, request := range verifyCases {
		if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("request=%#v error=%v", request, err)
		}
	}
	unsigned := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	if err := client.Verify(context.Background(), unsigned, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("unsigned request=%v", err)
	}
	badSignature := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	badSignature.Header.Set("X-Hub-Signature-256", "sha256=00")
	if err := client.Verify(context.Background(), badSignature, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("bad signature=%v", err)
	}
	if err := client.Verify(context.Background(), signedWebhookRequest(body, "wrong-secret"), body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong secret=%v", err)
	}
	_, withoutWebhook := newTestClient(t, server, false)
	if err := withoutWebhook.Verify(context.Background(), signedWebhookRequest(body, "app-secret"), body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured verify=%v", err)
	}

	invalidBodies := [][]byte{
		nil,
		[]byte(`{`),
		[]byte(`{"object":"user","entry":[{}]}`),
		[]byte(`{"object":"page","entry":[{"id":"999","time":1,"messaging":[]}]}`),
		[]byte(`{"object":"page","entry":[{"id":"123456789","time":1,"messaging":[{"sender":{"id":"1"},"recipient":{"id":"2"},"timestamp":1,"message":{"mid":"mid"}}]}]}`),
		[]byte(`{"object":"page","entry":[{"id":"123456789","time":1,"messaging":[{"sender":{"id":"1"},"recipient":{"id":"123456789"},"timestamp":1,"message":{}}]}]}`),
		[]byte(`{"object":"page","entry":[{"id":"123456789","time":1,"messaging":[{"sender":{"id":"1"},"recipient":{"id":"123456789"},"timestamp":1,"reaction":{"mid":"mid","action":"invalid"}}]}]}`),
	}
	for index, invalid := range invalidBodies {
		if _, err := client.Decode(context.Background(), nil, invalid); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("invalid body %d error=%v", index, err)
		}
	}

	badMethod := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	if status, _, err := client.HandleChallenge(context.Background(), badMethod); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("challenge method status=%d err=%v", status, err)
	}
	badToken := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=x", nil)
	if status, _, err := client.HandleChallenge(context.Background(), badToken); status != http.StatusForbidden || !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("challenge token status=%d err=%v", status, err)
	}
	emptyChallenge := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token", nil)
	if status, _, err := client.HandleChallenge(context.Background(), emptyChallenge); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty challenge status=%d err=%v", status, err)
	}
	if status, _, err := withoutWebhook.HandleChallenge(context.Background(), emptyChallenge); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured challenge status=%d err=%v", status, err)
	}
}
