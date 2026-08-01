package line

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 8 << 20

type webhookEnvelope struct {
	Destination string            `json:"destination"`
	Events      []json.RawMessage `json:"events"`
}

type webhookWire struct {
	Type            string          `json:"type"`
	Mode            string          `json:"mode"`
	Timestamp       int64           `json:"timestamp"`
	WebhookEventID  string          `json:"webhookEventId"`
	Source          *EventSource    `json:"source"`
	ReplyToken      string          `json:"replyToken"`
	Message         json.RawMessage `json:"message"`
	Postback        json.RawMessage `json:"postback"`
	DeliveryContext struct {
		IsRedelivery bool `json:"isRedelivery"`
	} `json:"deliveryContext"`
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.channelSecret == "" {
		return unsupported("webhook_verify", "secret_ref is not configured with the LINE channel secret")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "LINE webhook must be a bounded, non-empty POST body")
	}
	provided, err := base64.StdEncoding.DecodeString(request.Header.Get("X-Line-Signature"))
	if err != nil || len(provided) != sha256.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.channelSecret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "LINE webhook body must be bounded and non-empty")
	}
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validLINEID(envelope.Destination, 'U') {
		return nil, invalidArgument("webhook_decode", "webhook destination must be a LINE bot user ID")
	}
	if c.botUserID != "" && envelope.Destination != c.botUserID {
		return nil, invalidArgument("webhook_decode", "webhook destination does not match account.settings.bot_user_id")
	}
	events := make([]socialhub.Event, 0, len(envelope.Events))
	for _, raw := range envelope.Events {
		var wire webhookWire
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if strings.TrimSpace(wire.Type) == "" || !validOpaque(wire.WebhookEventID, 128) || wire.Timestamp < 0 {
			return nil, invalidArgument("webhook_decode", "event type, webhook event ID, and non-negative timestamp are required")
		}
		payload := WebhookEvent{
			Destination: envelope.Destination, ID: wire.WebhookEventID, Type: wire.Type, Mode: wire.Mode,
			Timestamp: millisecondTime(wire.Timestamp), Source: wire.Source, ReplyToken: wire.ReplyToken,
			IsRedelivery: wire.DeliveryContext.IsRedelivery, Raw: append(json.RawMessage(nil), raw...),
		}
		if wire.Type == "message" {
			if len(wire.Message) == 0 {
				return nil, invalidArgument("webhook_decode", "message event payload is required")
			}
			var message IncomingMessage
			if err := json.Unmarshal(wire.Message, &message); err != nil || !validOpaque(message.ID, 256) || strings.TrimSpace(message.Type) == "" {
				return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			message.Raw = append(json.RawMessage(nil), wire.Message...)
			payload.Message = &message
		}
		if wire.Type == "postback" {
			if len(wire.Postback) == 0 {
				return nil, invalidArgument("webhook_decode", "postback event payload is required")
			}
			var postback PostbackContent
			if err := json.Unmarshal(wire.Postback, &postback); err != nil || strings.TrimSpace(postback.Data) == "" {
				return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			payload.Postback = &postback
		}
		events = append(events, socialhub.Event{
			ID: wire.WebhookEventID, Type: eventType(wire, payload.Message), Platform: "line", AccountID: c.accountID, Payload: payload,
		})
	}
	return events, nil
}

func eventType(wire webhookWire, message *IncomingMessage) string {
	if wire.Type == "message" && message != nil {
		return "line.message." + message.Type
	}
	return "line." + wire.Type
}

func millisecondTime(value int64) *time.Time {
	parsed := time.UnixMilli(value).UTC()
	return &parsed
}

var _ socialhub.WebhookHandler = (*Client)(nil)
