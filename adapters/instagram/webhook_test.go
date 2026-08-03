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
	"time"

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

func signedInstagramWebhookRequest(body []byte, secret string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "https://app.example/webhook", nil)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return request
}

func TestMessagingWebhookEventsAndNormalization(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, nil, true)
	body := []byte(`{"object":"instagram","entry":[{"id":"178","time":1785672000000,"changes":[{"field":"comments","value":{"id":"comment-1"}}],"messaging":[` +
		`{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1785672000001,"message":{"mid":"mid.inbound","text":"hello","reply_to":{"mid":"mid.parent"},"attachments":[{"type":"image","payload":{"url":"https://cdn.example/photo.jpg"}},{"type":"file","payload":{"url":"http://invalid.example/file"}}]}},` +
		`{"sender":{"id":"178"},"recipient":{"id":"111"},"timestamp":1785672000002,"message":{"mid":"mid.echo","text":"sent","is_echo":true}},` +
		`{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1785672000003,"read":{"mid":"mid.echo"}},` +
		`{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1785672000004,"reaction":{"mid":"mid.echo","action":"react","reaction":"love","emoji":"heart"}},` +
		`{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1785672000005,"postback":{"mid":"mid.postback","title":"Start","payload":"GET_STARTED"}},` +
		`{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1785672000006,"referral":{"ref":"campaign"}},` +
		`{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1785672000007,"optin":{"ref":"campaign"}}]}]}`)
	request := signedInstagramWebhookRequest(body, "webhook-secret")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 8 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	wantTypes := []string{
		"instagram.comments", "instagram.messaging.message", "instagram.messaging.message", "instagram.messaging.read",
		"instagram.messaging.reaction", "instagram.messaging.postback", "instagram.messaging.referral", "instagram.messaging.event",
	}
	seen := make(map[string]struct{}, len(events))
	for index, event := range events {
		if event.Type != wantTypes[index] || event.ID == "" || event.Platform != "instagram" || event.AccountID != "brand" {
			t.Fatalf("event %d=%#v", index, event)
		}
		if _, duplicate := seen[event.ID]; duplicate {
			t.Fatalf("duplicate event ID=%q", event.ID)
		}
		seen[event.ID] = struct{}{}
	}
	inbound := events[1].Payload.(MessagingWebhookEvent)
	wantTime := time.UnixMilli(1785672000001).UTC()
	if inbound.NormalizedMessage == nil || inbound.NormalizedMessage.Direction != socialhub.DirectionInbound ||
		inbound.NormalizedMessage.ConversationID != "111" || inbound.NormalizedMessage.SenderID == nil ||
		*inbound.NormalizedMessage.SenderID != "111" || inbound.NormalizedMessage.ReplyToID == nil ||
		*inbound.NormalizedMessage.ReplyToID != "mid.parent" || len(inbound.NormalizedMessage.Media) != 1 ||
		inbound.NormalizedMessage.Media[0].Type != socialhub.MediaTypeImage || !inbound.Timestamp.Equal(wantTime) || len(inbound.Raw) == 0 {
		t.Fatalf("inbound=%#v", inbound)
	}
	echo := events[2].Payload.(MessagingWebhookEvent).NormalizedMessage
	if echo == nil || echo.Direction != socialhub.DirectionOutbound || echo.ConversationID != "111" {
		t.Fatalf("echo=%#v", echo)
	}
	if read := events[3].Payload.(MessagingWebhookEvent); read.Read == nil || read.Read.MessageID != "mid.echo" {
		t.Fatalf("read=%#v", read)
	}
	if reaction := events[4].Payload.(MessagingWebhookEvent); reaction.Reaction == nil || reaction.Reaction.Action != "react" {
		t.Fatalf("reaction=%#v", reaction)
	}
	if postback := events[5].Payload.(MessagingWebhookEvent); postback.Postback == nil || postback.Postback.Payload != "GET_STARTED" {
		t.Fatalf("postback=%#v", postback)
	}
	if referral := events[6].Payload.(MessagingWebhookEvent); len(referral.Referral) == 0 {
		t.Fatalf("referral=%#v", referral)
	}
	if generic := events[7].Payload.(MessagingWebhookEvent); generic.Message != nil || generic.NormalizedMessage != nil || len(generic.Raw) == 0 {
		t.Fatalf("generic=%#v", generic)
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

func TestWebhookRejectsInvalidVerificationPayloadsAndChallenges(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestAdapter(t, server, nil, true)
	body := []byte(`{"object":"instagram","entry":[{"id":"178","time":1,"messaging":[]}]}`)
	for _, request := range []*http.Request{nil, httptest.NewRequest(http.MethodGet, "/webhook", nil)} {
		if err := client.Verify(context.Background(), request, body); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("request=%#v error=%v", request, err)
		}
	}
	if err := client.Verify(context.Background(), httptest.NewRequest(http.MethodPost, "/webhook", nil), nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty body error=%v", err)
	}
	if err := client.Verify(context.Background(), httptest.NewRequest(http.MethodPost, "/webhook", nil), make([]byte, maxWebhookBodyBytes+1)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("large body error=%v", err)
	}
	unsigned := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	if err := client.Verify(context.Background(), unsigned, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("unsigned error=%v", err)
	}
	if err := client.Verify(context.Background(), signedInstagramWebhookRequest(body, "wrong"), body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong signature error=%v", err)
	}
	_, unconfigured := newTestAdapter(t, server, nil, false)
	if err := unconfigured.Verify(context.Background(), signedInstagramWebhookRequest(body, "webhook-secret"), body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured verify error=%v", err)
	}

	invalidBodies := [][]byte{
		nil,
		[]byte(`{`),
		[]byte(`{"object":"page","entry":[{}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"999","time":1}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":0}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":1,"changes":[{"field":"","value":null}]}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":1,"messaging":[null]}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":1,"messaging":[{"sender":{"id":"111"},"recipient":{"id":"222"},"timestamp":1,"message":{"mid":"mid"}}]}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":1,"messaging":[{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1,"message":{}}]}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":1,"messaging":[{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1,"read":{}}]}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":1,"messaging":[{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1,"reaction":{"mid":"mid","action":"toggle"}}]}]}`),
		[]byte(`{"object":"instagram","entry":[{"id":"178","time":1,"messaging":[{"sender":{"id":"111"},"recipient":{"id":"178"},"timestamp":1,"postback":{}}]}]}`),
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
	empty := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token", nil)
	if status, _, err := client.HandleChallenge(context.Background(), empty); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty challenge status=%d err=%v", status, err)
	}
	if status, _, err := unconfigured.HandleChallenge(context.Background(), empty); status != http.StatusBadRequest || !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured challenge status=%d err=%v", status, err)
	}
}
