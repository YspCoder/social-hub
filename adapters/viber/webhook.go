package viber

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 8 << 20

type webhookWire struct {
	Event        string           `json:"event"`
	Timestamp    int64            `json:"timestamp"`
	MessageToken json.Number      `json:"message_token"`
	UserID       string           `json:"user_id"`
	User         *UserDetails     `json:"user"`
	Sender       *UserDetails     `json:"sender"`
	Message      *IncomingMessage `json:"message"`
	Type         string           `json:"type"`
	Context      string           `json:"context"`
	Subscribed   *bool            `json:"subscribed"`
	Description  string           `json:"desc"`
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Viber webhook must be a bounded, non-empty POST body")
	}
	provided, err := hex.DecodeString(strings.TrimSpace(request.Header.Get("X-Viber-Content-Signature")))
	if err != nil || len(provided) != sha256.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.authToken))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "Viber webhook body must be bounded and non-empty")
	}
	var wire webhookWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	token := wire.MessageToken.String()
	if !validEventName(wire.Event) || wire.Timestamp < 0 || !validMessageToken(token) {
		return nil, invalidArgument("webhook_decode", "event, non-negative timestamp, and numeric message token are required")
	}
	if wire.UserID != "" && !validOpaqueID(wire.UserID) {
		return nil, invalidArgument("webhook_decode", "callback user ID is invalid")
	}
	payload := WebhookEvent{
		Event: wire.Event, Timestamp: time.UnixMilli(wire.Timestamp).UTC(), MessageToken: token,
		UserID: wire.UserID, User: wire.User, Sender: wire.Sender, Message: wire.Message,
		ConversationType: wire.Type, Context: wire.Context, Subscribed: wire.Subscribed,
		Description: wire.Description, Raw: append(json.RawMessage(nil), body...),
	}
	eventType := "viber." + wire.Event
	if wire.Event == "message" {
		if wire.Sender == nil || !validOpaqueID(wire.Sender.ID) || wire.Message == nil || !validEventName(wire.Message.Type) {
			return nil, invalidArgument("webhook_decode", "message events require a sender and typed message payload")
		}
		payload.NormalizedMessage = c.mapInboundMessage(token, payload.Timestamp, wire.Sender, wire.Message)
		eventType += "." + wire.Message.Type
	}
	for _, user := range []*UserDetails{wire.User, wire.Sender} {
		if user != nil && (!validOpaqueID(user.ID) || (user.Avatar != "" && !validRemoteURL(user.Avatar, 2000))) {
			return nil, invalidArgument("webhook_decode", "callback user data is invalid")
		}
	}
	return []socialhub.Event{{
		ID: wire.Event + ":" + token, Type: eventType, Platform: "viber", AccountID: c.accountID, Payload: payload,
	}}, nil
}

// SetWebhook configures the callback URL and optional event filtering.
func (c *Client) SetWebhook(ctx context.Context, input SetWebhookRequest, options ...socialhub.CallOption) (*WebhookResult, error) {
	if !validWebhookURL(input.URL) {
		return nil, invalidArgument("set_webhook", "webhook URL must be an absolute HTTPS URL without credentials or fragments")
	}
	body := map[string]any{"url": input.URL}
	if input.EventTypes != nil {
		events, err := validateEventTypes(input.EventTypes)
		if err != nil {
			return nil, err
		}
		body["event_types"] = events
	}
	if input.SendName != nil {
		body["send_name"] = *input.SendName
	}
	if input.SendPhoto != nil {
		body["send_photo"] = *input.SendPhoto
	}
	var result WebhookResult
	if err := c.request(ctx, "/pa/set_webhook", body, &result, options...); err != nil {
		return nil, err
	}
	if _, err := validateEventTypes(result.EventTypes); err != nil {
		return nil, platformError("set_webhook", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &result, nil
}

// RemoveWebhook removes the configured callback endpoint.
func (c *Client) RemoveWebhook(ctx context.Context, options ...socialhub.CallOption) error {
	var result WebhookResult
	return c.request(ctx, "/pa/set_webhook", map[string]string{"url": ""}, &result, options...)
}

func validateEventTypes(values []WebhookEventType) ([]WebhookEventType, error) {
	allowed := map[WebhookEventType]struct{}{
		WebhookDelivered: {}, WebhookSeen: {}, WebhookFailed: {}, WebhookSubscribed: {},
		WebhookUnsubscribed: {}, WebhookConversationStarted: {}, WebhookMessage: {},
	}
	result := make([]WebhookEventType, len(values))
	seen := make(map[WebhookEventType]struct{}, len(values))
	for index, value := range values {
		if _, ok := allowed[value]; !ok {
			return nil, invalidArgument("set_webhook", "event_types contains an unsupported Viber event")
		}
		if _, exists := seen[value]; exists {
			return nil, invalidArgument("set_webhook", "event_types must not contain duplicates")
		}
		seen[value], result[index] = struct{}{}, value
	}
	return result, nil
}

func validWebhookURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validEventName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return false
	}
	return true
}

func (c *Client) mapInboundMessage(token string, sentAt time.Time, sender *UserDetails, message *IncomingMessage) *socialhub.Message {
	var text *string
	if message.Text != "" {
		text = stringPointer(message.Text)
	} else if message.Type == "url" && message.Media != "" {
		text = stringPointer(message.Media)
	}
	var media []socialhub.Media
	switch message.Type {
	case "picture":
		media = []socialhub.Media{{URL: message.Media, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady}}
	case "video":
		duration := time.Duration(message.Duration) * time.Millisecond
		media = []socialhub.Media{{URL: message.Media, Type: socialhub.MediaTypeVideo, Size: positiveInt64Pointer(message.FileSize), Duration: &duration, State: socialhub.MediaStateReady}}
	case "file":
		media = []socialhub.Media{{URL: message.Media, Type: socialhub.MediaTypeDocument, Size: positiveInt64Pointer(message.FileSize), State: socialhub.MediaStateReady}}
	}
	extension, _ := json.Marshal(message)
	senderID := sender.ID
	return &socialhub.Message{
		Platform: "viber", AccountID: c.accountID, ID: token, ConversationID: sender.ID,
		SenderID: &senderID, Text: text, Media: media, SentAt: &sentAt, Direction: socialhub.DirectionInbound,
		Extensions: map[string]json.RawMessage{"viber.message": extension},
	}
}

func positiveInt64Pointer(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}
