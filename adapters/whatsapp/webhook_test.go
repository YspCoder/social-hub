package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"social-hub/pkg/socialhub"
)

func signedWebhookRequest(body []byte, secret string) *http.Request {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	request := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return request
}

func TestWebhookVerifyChallengeAndBatchDecode(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"987654321","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"display_phone_number":"15551234567","phone_number_id":"123456789"},"contacts":[{"profile":{"name":"Ada"},"wa_id":"15550001111"}],"messages":[{"from":"15550001111","id":"wamid.text","timestamp":"1785585600","type":"text","text":{"body":"hello"}},{"from":"15550002222","id":"wamid.image","timestamp":"1785585601","type":"image","context":{"id":"wamid.parent"},"image":{"id":"media-1","mime_type":"image/jpeg"}}],"statuses":[{"id":"wamid.outbound","status":"delivered","timestamp":"1785585602","recipient_id":"15550003333","conversation":{"id":"conversation-1"},"pricing":{"billable":true},"errors":[]},{"id":"wamid.outbound","status":"read","timestamp":"1785585603","recipient_id":"15550003333"}]}}]}]}`)
	request := signedWebhookRequest(body, "app-secret")
	if err := client.Verify(context.Background(), request, body); err != nil {
		t.Fatal(err)
	}
	events, err := client.Decode(context.Background(), request, body)
	if err != nil || len(events) != 4 {
		t.Fatalf("events=%#v error=%v", events, err)
	}
	if events[0].ID != "wamid.text" || events[0].Type != "whatsapp.message.text" || events[0].Platform != "whatsapp" || events[0].AccountID != "main" {
		t.Fatalf("message event=%#v", events[0])
	}
	message, ok := events[0].Payload.(MessageWebhookPayload)
	if !ok || message.Contact == nil || message.Contact.Name != "Ada" || message.From != "15550001111" || message.Timestamp == nil || len(message.Raw) == 0 || !strings.Contains(string(message.Raw), `"body":"hello"`) {
		t.Fatalf("message payload=%#v", events[0].Payload)
	}
	reply, ok := events[1].Payload.(MessageWebhookPayload)
	if !ok || reply.ReplyToID != "wamid.parent" || reply.Type != "image" {
		t.Fatalf("reply payload=%#v", events[1].Payload)
	}
	status, ok := events[2].Payload.(StatusWebhookPayload)
	if !ok || events[2].Type != "whatsapp.status.delivered" || status.RecipientID != "15550003333" || len(status.Conversation) == 0 || len(status.Pricing) == 0 || len(status.Raw) == 0 {
		t.Fatalf("status payload=%#v", events[2].Payload)
	}
	if events[2].ID == events[3].ID {
		t.Fatalf("status IDs must differ: %q", events[2].ID)
	}

	challenge := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=challenge-value", nil)
	statusCode, response, err := client.HandleChallenge(context.Background(), challenge)
	if err != nil || statusCode != http.StatusOK || string(response) != "challenge-value" {
		t.Fatalf("challenge status=%d body=%q error=%v", statusCode, response, err)
	}
}

func TestWebhookRoutingAndGenericMessageChange(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	nonMessage := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"987654321","changes":[{"field":"message_template_status_update","value":{"event":"APPROVED"}}]}]}`)
	events, err := client.Decode(context.Background(), nil, nonMessage)
	if err != nil || len(events) != 0 {
		t.Fatalf("non-message events=%#v error=%v", events, err)
	}
	emptyMessage := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"987654321","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"123456789"},"errors":[{"code":131000}]}}]}]}`)
	events, err = client.Decode(context.Background(), nil, emptyMessage)
	if err != nil || len(events) != 1 || events[0].Type != "whatsapp.change.messages" || events[0].ID == "" {
		t.Fatalf("generic events=%#v error=%v", events, err)
	}
	if raw, ok := events[0].Payload.(json.RawMessage); !ok || !strings.Contains(string(raw), `"errors"`) {
		t.Fatalf("generic payload=%#v", events[0].Payload)
	}

	wrongPhone := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"987654321","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"999"}}}]}]}`)
	if _, err := client.Decode(context.Background(), nil, wrongPhone); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("wrong phone error=%v", err)
	}
	wrongBusiness := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"111","changes":[]}]}`)
	if _, err := client.Decode(context.Background(), nil, wrongBusiness); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("wrong business error=%v", err)
	}
}

func TestWebhookRejectsInvalidVerificationAndPayloads(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := newTestClient(t, server, allScopes(), true)
	body := []byte(`{"object":"whatsapp_business_account","entry":[]}`)
	cases := []*http.Request{
		nil,
		httptest.NewRequest(http.MethodGet, "/webhook", nil),
		httptest.NewRequest(http.MethodPost, "/webhook", nil),
	}
	for _, request := range cases {
		if err := client.Verify(context.Background(), request, body); err == nil {
			t.Fatalf("request=%#v should fail", request)
		}
	}
	badSignature := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	badSignature.Header.Set("X-Hub-Signature-256", "sha256=00")
	if err := client.Verify(context.Background(), badSignature, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("signature error=%v", err)
	}
	wrongSecret := signedWebhookRequest(body, "wrong-secret")
	if err := client.Verify(context.Background(), wrongSecret, body); !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("wrong secret error=%v", err)
	}
	invalidPayloads := [][]byte{
		nil,
		[]byte(`{`),
		[]byte(`{"object":"page","entry":[{}]}`),
		[]byte(`{"object":"whatsapp_business_account","entry":[{"id":""}]}`),
		[]byte(`{"object":"whatsapp_business_account","entry":[{"id":"987654321","changes":[{"field":"messages","value":{}}]}]}`),
	}
	for _, payload := range invalidPayloads {
		if _, err := client.Decode(context.Background(), nil, payload); err == nil {
			t.Fatalf("payload=%q should fail", payload)
		}
	}
	badChallenge := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=x", nil)
	if status, _, err := client.HandleChallenge(context.Background(), badChallenge); status != http.StatusForbidden || !errors.Is(err, socialhub.ErrPermissionDenied) {
		t.Fatalf("challenge status=%d error=%v", status, err)
	}
	withoutSecret := newTestClient(t, server, allScopes(), false)
	if err := withoutSecret.Verify(context.Background(), signedWebhookRequest(body, "app-secret"), body); !errors.Is(err, socialhub.ErrUnsupported) {
		t.Fatalf("unconfigured verify error=%v", err)
	}
}
