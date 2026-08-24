package messenger

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

const (
	maxWebhookBodyBytes = 8 << 20
	maxChallengeBytes   = 4096
)

type webhookEnvelope struct {
	Object string         `json:"object"`
	Entry  []webhookEntry `json:"entry"`
}

type webhookEntry struct {
	ID        string            `json:"id"`
	Time      int64             `json:"time"`
	Messaging []json.RawMessage `json:"messaging"`
}

type messagingWire struct {
	Sender    EventParty       `json:"sender"`
	Recipient EventParty       `json:"recipient"`
	Timestamp int64            `json:"timestamp"`
	Message   *IncomingMessage `json:"message,omitempty"`
	Delivery  *DeliveryReceipt `json:"delivery,omitempty"`
	Read      *ReadReceipt     `json:"read,omitempty"`
	Postback  *Postback        `json:"postback,omitempty"`
	Reaction  *Reaction        `json:"reaction,omitempty"`
}

// Verify authenticates one Messenger Platform callback using the Meta app
// secret and the unmodified request body.
func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookSecret == "" {
		return unsupported("webhook_verify", "webhook.secret_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Messenger webhook must be a bounded, non-empty POST body")
	}
	signature := strings.TrimSpace(request.Header.Get("X-Hub-Signature-256"))
	if !strings.HasPrefix(signature, "sha256=") {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

// Decode validates and normalizes Page messaging events. Call Verify first and
// acknowledge successful deliveries promptly in the HTTP integration layer.
func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "Messenger webhook body must be bounded and non-empty")
	}
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.Object != "page" || len(envelope.Entry) == 0 {
		return nil, invalidArgument("webhook_decode", "webhook object must be page with at least one entry")
	}

	events := make([]socialhub.Event, 0)
	for _, entry := range envelope.Entry {
		if entry.ID != c.pageID || entry.Time <= 0 {
			return nil, invalidArgument("webhook_decode", "webhook Page ID or entry timestamp does not match the configured account")
		}
		entryTime := parseWebhookTimestamp(entry.Time)
		for _, raw := range entry.Messaging {
			if len(raw) == 0 || string(raw) == "null" {
				return nil, invalidArgument("webhook_decode", "messaging event payload is required")
			}
			var wire messagingWire
			if err := json.Unmarshal(raw, &wire); err != nil {
				return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			if !validNumericID(wire.Sender.ID) || !validNumericID(wire.Recipient.ID) ||
				(wire.Sender.ID == c.pageID) == (wire.Recipient.ID == c.pageID) || wire.Timestamp <= 0 {
				return nil, invalidArgument("webhook_decode", "messaging event must identify one configured Page party, one PSID, and a timestamp")
			}

			eventTime := parseWebhookTimestamp(wire.Timestamp)
			payload := WebhookEvent{
				PageID: c.pageID, EntryTime: entryTime, Sender: wire.Sender, Recipient: wire.Recipient,
				Timestamp: eventTime, Message: wire.Message, Delivery: wire.Delivery, Read: wire.Read,
				Postback: wire.Postback, Reaction: wire.Reaction, Raw: append(json.RawMessage(nil), raw...),
			}
			eventType, eventID, err := c.classifyWebhookEvent(&payload, wire)
			if err != nil {
				return nil, err
			}
			if eventID == "" {
				eventID = webhookEventID(entry.ID, entry.Time, raw)
			}
			events = append(events, socialhub.Event{
				ID: eventID, Type: eventType, Platform: "facebook", AccountID: c.accountID, Payload: payload,
			})
		}
	}
	return events, nil
}

func (c *Client) classifyWebhookEvent(payload *WebhookEvent, wire messagingWire) (string, string, error) {
	switch {
	case wire.Message != nil:
		if !validOpaqueID(wire.Message.ID, 512) {
			return "", "", invalidArgument("webhook_decode", "message event mid is required and must be valid")
		}
		payload.NormalizedMessage = c.normalizeWebhookMessage(wire, payload.Timestamp)
		return "facebook.messenger.message", wire.Message.ID, nil
	case wire.Delivery != nil:
		if wire.Delivery.Watermark <= 0 && len(wire.Delivery.MessageIDs) == 0 {
			return "", "", invalidArgument("webhook_decode", "delivery event requires mids or a watermark")
		}
		for _, messageID := range wire.Delivery.MessageIDs {
			if !validOpaqueID(messageID, 512) {
				return "", "", invalidArgument("webhook_decode", "delivery event contains an invalid mid")
			}
		}
		return "facebook.messenger.delivery", "", nil
	case wire.Read != nil:
		if wire.Read.Watermark <= 0 {
			return "", "", invalidArgument("webhook_decode", "read event requires a positive watermark")
		}
		return "facebook.messenger.read", "", nil
	case wire.Postback != nil:
		if wire.Postback.MessageID != "" && !validOpaqueID(wire.Postback.MessageID, 512) {
			return "", "", invalidArgument("webhook_decode", "postback event contains an invalid mid")
		}
		if strings.TrimSpace(wire.Postback.Title) == "" && strings.TrimSpace(wire.Postback.Payload) == "" && len(wire.Postback.Referral) == 0 {
			return "", "", invalidArgument("webhook_decode", "postback event requires title, payload, or referral data")
		}
		return "facebook.messenger.postback", "", nil
	case wire.Reaction != nil:
		if !validOpaqueID(wire.Reaction.MessageID, 512) || (wire.Reaction.Action != "react" && wire.Reaction.Action != "unreact") {
			return "", "", invalidArgument("webhook_decode", "reaction event requires a valid mid and react or unreact action")
		}
		return "facebook.messenger.reaction", "", nil
	default:
		return "facebook.messenger.event", "", nil
	}
}

func (c *Client) normalizeWebhookMessage(wire messagingWire, sentAt time.Time) *socialhub.Message {
	message := wire.Message
	direction := socialhub.DirectionInbound
	conversationID := wire.Sender.ID
	if wire.Sender.ID == c.pageID {
		direction = socialhub.DirectionOutbound
		conversationID = wire.Recipient.ID
	}
	senderID := wire.Sender.ID
	result := &socialhub.Message{
		Platform: "facebook", AccountID: c.accountID, ID: message.ID, ConversationID: conversationID,
		SenderID: &senderID, RecipientIDs: []string{wire.Recipient.ID}, SentAt: &sentAt, Direction: direction,
	}
	if message.Text != "" {
		result.Text = stringPointer(message.Text)
	}
	if message.ReplyTo != nil && validOpaqueID(message.ReplyTo.ID, 512) {
		result.ReplyToID = stringPointer(message.ReplyTo.ID)
	}
	for _, attachment := range message.Attachments {
		var attachmentPayload struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(attachment.Payload, &attachmentPayload) != nil || !validRemoteURL(attachmentPayload.URL) {
			continue
		}
		extension, _ := json.Marshal(attachment)
		result.Media = append(result.Media, socialhub.Media{
			URL: attachmentPayload.URL, Type: normalizedWebhookAttachmentType(attachment.Type), State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"facebook.messenger_attachment": extension},
		})
	}
	extension, _ := json.Marshal(message)
	result.Extensions = map[string]json.RawMessage{"facebook.messenger_message": extension}
	return result
}

func normalizedWebhookAttachmentType(value string) socialhub.MediaType {
	switch value {
	case "image", "sticker":
		return socialhub.MediaTypeImage
	case "audio":
		return socialhub.MediaTypeAudio
	case "video":
		return socialhub.MediaTypeVideo
	default:
		return socialhub.MediaTypeDocument
	}
}

// HandleChallenge validates Meta's GET subscription handshake and returns the
// challenge body that an HTTP integration must echo unchanged.
func (c *Client) HandleChallenge(_ context.Context, request *http.Request) (int, []byte, error) {
	if c.webhookToken == "" {
		return http.StatusBadRequest, nil, unsupported("webhook_challenge", "webhook.token_ref is not configured")
	}
	if request == nil || request.Method != http.MethodGet {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "Messenger webhook challenge must use GET")
	}
	query := request.URL.Query()
	if query.Get("hub.mode") != "subscribe" || !hmac.Equal([]byte(query.Get("hub.verify_token")), []byte(c.webhookToken)) {
		return http.StatusForbidden, nil, platformError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	challenge := query.Get("hub.challenge")
	if challenge == "" || len(challenge) > maxChallengeBytes {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "hub.challenge must be non-empty and bounded")
	}
	return http.StatusOK, []byte(challenge), nil
}

func parseWebhookTimestamp(value int64) time.Time {
	if value >= 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}

func webhookEventID(pageID string, entryTime int64, raw json.RawMessage) string {
	canonical := raw
	var value any
	if json.Unmarshal(raw, &value) == nil {
		if encoded, err := json.Marshal(value); err == nil {
			canonical = encoded
		}
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(pageID))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(strconv.FormatInt(entryTime, 10)))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(canonical)
	return hex.EncodeToString(digest.Sum(nil))
}

var _ socialhub.WebhookHandler = (*Client)(nil)
var _ socialhub.ChallengeHandler = (*Client)(nil)
