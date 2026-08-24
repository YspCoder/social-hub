package zalo

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 8 << 20

type webhookWire struct {
	AppID       string          `json:"app_id"`
	Sender      EventParty      `json:"sender"`
	Recipient   EventParty      `json:"recipient"`
	UserIDByApp string          `json:"user_id_by_app"`
	EventName   string          `json:"event_name"`
	Message     json.RawMessage `json:"message"`
	Timestamp   string          `json:"timestamp"`
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.appID == "" || c.webhookSecret == "" {
		return unsupported("webhook_verify", "app_id and webhook.secret_ref are required")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Zalo webhook must be a bounded, non-empty POST body")
	}
	var wire webhookWire
	if err := json.Unmarshal(body, &wire); err != nil || wire.AppID != c.appID || !validTimestamp(wire.Timestamp) {
		return invalidArgument("webhook_verify", "webhook app_id and timestamp are invalid")
	}
	provided := strings.TrimSpace(request.Header.Get("X-ZEvent-Signature"))
	provided = strings.TrimPrefix(provided, "mac=")
	digest, err := hex.DecodeString(provided)
	if err != nil || len(digest) != sha256.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(wire.AppID))
	_, _ = hash.Write(body)
	_, _ = hash.Write([]byte(wire.Timestamp))
	_, _ = hash.Write([]byte(c.webhookSecret))
	if !hmac.Equal(digest, hash.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "Zalo webhook body must be bounded and non-empty")
	}
	var wire webhookWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validNumericID(wire.AppID) || (c.appID != "" && wire.AppID != c.appID) ||
		!validNumericID(wire.Sender.ID) || !validNumericID(wire.Recipient.ID) || !validOpaqueID(wire.EventName, 128) || !validTimestamp(wire.Timestamp) {
		return nil, invalidArgument("webhook_decode", "webhook identity, event name, or timestamp is invalid")
	}
	if c.oaID != "" && wire.Sender.ID != c.oaID && wire.Recipient.ID != c.oaID {
		return nil, invalidArgument("webhook_decode", "webhook does not reference account.settings.oa_id")
	}
	timestamp, err := parseMilliseconds(wire.Timestamp)
	if err != nil {
		return nil, err
	}
	retryCount := 0
	if request != nil && request.Header.Get("num_retry") != "" {
		retryCount, err = strconv.Atoi(request.Header.Get("num_retry"))
		if err != nil || retryCount < 0 {
			return nil, invalidArgument("webhook_decode", "num_retry must be a non-negative integer")
		}
	}
	payload := WebhookEvent{
		AppID: wire.AppID, EventName: wire.EventName, Sender: wire.Sender, Recipient: wire.Recipient,
		UserIDByApp: wire.UserIDByApp, Timestamp: timestamp, RetryCount: retryCount,
		Raw: append(json.RawMessage(nil), body...),
	}
	eventID := wire.EventName + ":" + wire.Timestamp + ":" + wire.Sender.ID + ":" + wire.Recipient.ID
	if len(wire.Message) > 0 && string(wire.Message) != "null" {
		var message IncomingMessage
		if err := json.Unmarshal(wire.Message, &message); err != nil || !validOpaqueID(message.ID, 256) {
			return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		payload.Message = &message
		payload.NormalizedMessage = c.normalizeMessage(wire, message, timestamp)
		eventID = wire.EventName + ":" + message.ID
	}
	return []socialhub.Event{{
		ID: eventID, Type: "zalo." + wire.EventName, Platform: "zalo", AccountID: c.accountID, Payload: payload,
	}}, nil
}

func (c *Client) normalizeMessage(wire webhookWire, message IncomingMessage, timestamp time.Time) *socialhub.Message {
	inbound := strings.HasPrefix(wire.EventName, "user_send_")
	conversationID, senderID, recipientID, direction := wire.Sender.ID, wire.Sender.ID, wire.Recipient.ID, socialhub.DirectionInbound
	if !inbound {
		conversationID, direction = wire.Recipient.ID, socialhub.DirectionOutbound
	}
	result := &socialhub.Message{
		Platform: "zalo", AccountID: c.accountID, ID: message.ID, ConversationID: conversationID,
		SenderID: &senderID, RecipientIDs: []string{recipientID}, SentAt: &timestamp, Direction: direction,
	}
	if message.Text != "" {
		result.Text = &message.Text
	}
	if message.QuoteMsgID != "" {
		result.ReplyToID = &message.QuoteMsgID
	}
	for _, attachment := range message.Attachments {
		var content struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(attachment.Payload, &content)
		if content.URL == "" {
			continue
		}
		mediaType := socialhub.MediaTypeDocument
		switch attachment.Type {
		case "image", "sticker":
			mediaType = socialhub.MediaTypeImage
		case "gif":
			mediaType = socialhub.MediaTypeAnimation
		case "video":
			mediaType = socialhub.MediaTypeVideo
		case "audio":
			mediaType = socialhub.MediaTypeAudio
		case "file":
			mediaType = socialhub.MediaTypeDocument
		default:
			continue
		}
		extension, _ := json.Marshal(attachment)
		result.Media = append(result.Media, socialhub.Media{
			URL: content.URL, Type: mediaType, State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"zalo.attachment": extension},
		})
	}
	return result
}

func validTimestamp(value string) bool {
	_, err := parseMilliseconds(value)
	return err == nil
}
