package discourse

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 8 << 20

// WebhookPayload preserves the official Discourse event headers and raw JSON
// body. Payload schemas vary by selected event type and installed plugins.
type WebhookPayload struct {
	EventID   string
	EventType string
	Event     string
	Raw       json.RawMessage
}

func (client *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if client.webhookSecret == "" {
		return unsupported("webhook_verify", "Discourse webhook secret is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Discourse webhook must be a bounded, non-empty POST body")
	}
	signature := strings.TrimSpace(request.Header.Get("X-Discourse-Event-Signature"))
	if !strings.HasPrefix(signature, "sha256=") {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(client.webhookSecret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (client *Client) Decode(ctx context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if err := client.Verify(ctx, request, body); err != nil {
		return nil, err
	}
	eventID := strings.TrimSpace(request.Header.Get("X-Discourse-Event-Id"))
	eventType := strings.TrimSpace(request.Header.Get("X-Discourse-Event-Type"))
	event := strings.TrimSpace(request.Header.Get("X-Discourse-Event"))
	if !validWebhookValue(eventID, 256) || !validWebhookValue(eventType, 128) || !validWebhookValue(event, 128) {
		return nil, invalidArgument("webhook_decode", "Discourse event ID, type, and event headers are required")
	}
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	payload := WebhookPayload{
		EventID: eventID, EventType: eventType, Event: event, Raw: append(json.RawMessage(nil), body...),
	}
	return []socialhub.Event{{
		ID: eventID, Type: "discourse." + event, Platform: "discourse", AccountID: client.accountID, Payload: payload,
	}}, nil
}

func validWebhookValue(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}
