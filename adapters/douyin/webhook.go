package douyin

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

// WebhookEvent preserves the standard Douyin event envelope.
type WebhookEvent struct {
	Event      string          `json:"event"`
	FromUserID string          `json:"from_user_id"`
	ToUserID   string          `json:"to_user_id"`
	ClientKey  string          `json:"client_key"`
	Content    json.RawMessage `json:"content"`
	MessageID  string          `json:"msg_id"`
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookSecret == "" {
		return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "douyin", Product: "openapi", Op: "webhook_verify", PlatformMessage: "client secret is not configured"}
	}
	hash := sha1.New()
	_, _ = hash.Write([]byte(c.webhookSecret))
	_, _ = hash.Write(body)
	expected := hex.EncodeToString(hash.Sum(nil))
	provided := request.Header.Get("X-Douyin-Signature")
	if provided == "" || !hmac.Equal([]byte(provided), []byte(expected)) {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if event.Event == "" {
		return nil, invalidArgument("webhook_decode", "event is required")
	}
	if c.clientKey != "" && event.ClientKey != c.clientKey {
		return nil, wrapError("webhook_decode", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	eventID := firstNonEmpty(request.Header.Get("Msg-Id"), event.MessageID)
	if eventID == "" {
		digest := sha1.Sum(body)
		eventID = hex.EncodeToString(digest[:])
	}
	return []socialhub.Event{{ID: eventID, Type: event.Event, Platform: "douyin", AccountID: c.accountID, Payload: event}}, nil
}

// ChallengeResponse creates the JSON response for a verify_webhook event.
func (c *Client) ChallengeResponse(body []byte) ([]byte, error) {
	var envelope struct {
		Event   string `json:"event"`
		Content struct {
			Challenge json.RawMessage `json:"challenge"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, wrapError("webhook_challenge", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.Event != "verify_webhook" || len(envelope.Content.Challenge) == 0 {
		return nil, invalidArgument("webhook_challenge", "verify_webhook challenge is required")
	}
	return json.Marshal(struct {
		Challenge json.RawMessage `json:"challenge"`
	}{Challenge: envelope.Content.Challenge})
}

var _ socialhub.WebhookHandler = (*Client)(nil)
