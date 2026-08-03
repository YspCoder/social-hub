package instagram

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
	Changes   []webhookChange   `json:"changes"`
	Messaging []json.RawMessage `json:"messaging"`
}

type webhookChange struct {
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
}

type messagingWire struct {
	Sender    WebhookParty     `json:"sender"`
	Recipient WebhookParty     `json:"recipient"`
	Timestamp int64            `json:"timestamp"`
	Message   *IncomingMessage `json:"message,omitempty"`
	Read      *ReadReceipt     `json:"read,omitempty"`
	Postback  *MessagePostback `json:"postback,omitempty"`
	Reaction  *MessageReaction `json:"reaction,omitempty"`
	Referral  json.RawMessage  `json:"referral,omitempty"`
}

// Verify authenticates an Instagram callback using the Meta app secret and
// the exact, unmodified POST body.
func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookSecret == "" {
		return unsupported("webhook_verify", "webhook.secret_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Instagram webhook must be a bounded, non-empty POST body")
	}
	signature := strings.TrimSpace(request.Header.Get("X-Hub-Signature-256"))
	if !strings.HasPrefix(signature, "sha256=") {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

// Decode validates and normalizes both Instagram changes and messaging
// callbacks. Call Verify first in the HTTP integration layer.
func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "Instagram webhook body must be bounded and non-empty")
	}
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.Object != "instagram" || len(envelope.Entry) == 0 {
		return nil, invalidArgument("webhook_decode", "webhook object must be instagram with at least one entry")
	}

	events := make([]socialhub.Event, 0)
	for _, entry := range envelope.Entry {
		if entry.ID != c.userID || entry.Time <= 0 {
			return nil, invalidArgument("webhook_decode", "webhook account ID or entry timestamp does not match the configured account")
		}
		for index, change := range entry.Changes {
			if strings.TrimSpace(change.Field) == "" || len(change.Value) == 0 || string(change.Value) == "null" {
				return nil, invalidArgument("webhook_decode", "change event requires a field and value")
			}
			raw, _ := json.Marshal(change)
			events = append(events, socialhub.Event{
				ID: webhookEventID(entry.ID, entry.Time, "change", index, raw), Type: "instagram." + change.Field,
				Platform: "instagram", AccountID: c.accountID, Payload: json.RawMessage(raw),
			})
		}
		entryTime := parseWebhookTimestamp(entry.Time)
		for index, raw := range entry.Messaging {
			event, err := c.decodeMessagingEvent(entry, entryTime, index, raw)
			if err != nil {
				return nil, err
			}
			events = append(events, event)
		}
	}
	return events, nil
}

func (c *Client) decodeMessagingEvent(entry webhookEntry, entryTime time.Time, index int, raw json.RawMessage) (socialhub.Event, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return socialhub.Event{}, invalidArgument("webhook_decode", "messaging event payload is required")
	}
	var wire messagingWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return socialhub.Event{}, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validMessagingID(wire.Sender.ID) || !validMessagingID(wire.Recipient.ID) ||
		(wire.Sender.ID == c.userID) == (wire.Recipient.ID == c.userID) || wire.Timestamp <= 0 {
		return socialhub.Event{}, invalidArgument("webhook_decode", "messaging event must identify one configured account party, one IGSID, and a timestamp")
	}
	payload := MessagingWebhookEvent{
		InstagramUserID: c.userID, EntryTime: entryTime, Sender: wire.Sender, Recipient: wire.Recipient,
		Timestamp: parseWebhookTimestamp(wire.Timestamp), Message: wire.Message, Read: wire.Read,
		Postback: wire.Postback, Reaction: wire.Reaction, Referral: append(json.RawMessage(nil), wire.Referral...),
		Raw: append(json.RawMessage(nil), raw...),
	}
	eventType, eventID, err := c.classifyMessagingEvent(&payload, wire)
	if err != nil {
		return socialhub.Event{}, err
	}
	if eventID == "" {
		eventID = webhookEventID(entry.ID, entry.Time, "messaging", index, raw)
	}
	return socialhub.Event{
		ID: eventID, Type: eventType, Platform: "instagram", AccountID: c.accountID, Payload: payload,
	}, nil
}

func (c *Client) classifyMessagingEvent(payload *MessagingWebhookEvent, wire messagingWire) (string, string, error) {
	switch {
	case wire.Message != nil:
		if !validMessageID(wire.Message.ID) {
			return "", "", invalidArgument("webhook_decode", "message event mid is required and must be valid")
		}
		payload.NormalizedMessage = c.normalizeWebhookMessage(wire, payload.Timestamp)
		return "instagram.messaging.message", wire.Message.ID, nil
	case wire.Read != nil:
		if !validMessageID(wire.Read.MessageID) {
			return "", "", invalidArgument("webhook_decode", "read event requires a valid mid")
		}
		return "instagram.messaging.read", "", nil
	case wire.Reaction != nil:
		if !validMessageID(wire.Reaction.MessageID) ||
			(wire.Reaction.Action != string(MessageReactionAdd) && wire.Reaction.Action != string(MessageReactionRemove)) {
			return "", "", invalidArgument("webhook_decode", "reaction event requires a valid mid and react or unreact action")
		}
		return "instagram.messaging.reaction", "", nil
	case wire.Postback != nil:
		if wire.Postback.MessageID != "" && !validMessageID(wire.Postback.MessageID) {
			return "", "", invalidArgument("webhook_decode", "postback event contains an invalid mid")
		}
		if strings.TrimSpace(wire.Postback.Title) == "" && strings.TrimSpace(wire.Postback.Payload) == "" && len(wire.Postback.Referral) == 0 {
			return "", "", invalidArgument("webhook_decode", "postback event requires title, payload, or referral data")
		}
		return "instagram.messaging.postback", "", nil
	case len(wire.Referral) > 0 && string(wire.Referral) != "null":
		return "instagram.messaging.referral", "", nil
	default:
		return "instagram.messaging.event", "", nil
	}
}

func (c *Client) normalizeWebhookMessage(wire messagingWire, sentAt time.Time) *socialhub.Message {
	direction := socialhub.DirectionInbound
	conversationID := wire.Sender.ID
	if wire.Sender.ID == c.userID {
		direction = socialhub.DirectionOutbound
		conversationID = wire.Recipient.ID
	}
	senderID := wire.Sender.ID
	result := &socialhub.Message{
		Platform: "instagram", AccountID: c.accountID, ID: wire.Message.ID, ConversationID: conversationID,
		SenderID: &senderID, RecipientIDs: []string{wire.Recipient.ID}, Text: stringPointer(wire.Message.Text),
		SentAt: &sentAt, Direction: direction,
	}
	if wire.Message.ReplyTo != nil {
		result.ReplyToID = stringPointer(firstNonEmpty(wire.Message.ReplyTo.ID, wire.Message.ReplyTo.MessageID))
	}
	for _, attachment := range wire.Message.Attachments {
		var value struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(attachment.Payload, &value) != nil || !validMessageMediaURL(value.URL) {
			continue
		}
		extension, _ := json.Marshal(attachment)
		result.Media = append(result.Media, socialhub.Media{
			URL: value.URL, Type: normalizedWebhookMediaType(attachment.Type), State: socialhub.MediaStateReady,
			Extensions: map[string]json.RawMessage{"instagram.messaging_attachment": extension},
		})
	}
	extension, _ := json.Marshal(wire.Message)
	result.Extensions = map[string]json.RawMessage{"instagram.messaging_message": extension}
	return result
}

func normalizedWebhookMediaType(value string) socialhub.MediaType {
	switch strings.ToLower(value) {
	case "image", "sticker", "share":
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
// challenge body for the HTTP integration to echo unchanged.
func (c *Client) HandleChallenge(_ context.Context, request *http.Request) (int, []byte, error) {
	if c.webhookToken == "" {
		return http.StatusBadRequest, nil, unsupported("webhook_challenge", "webhook.token_ref is not configured")
	}
	if request == nil || request.Method != http.MethodGet {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "Instagram webhook challenge must use GET")
	}
	query := request.URL.Query()
	if query.Get("hub.mode") != "subscribe" || !hmac.Equal([]byte(query.Get("hub.verify_token")), []byte(c.webhookToken)) {
		return http.StatusForbidden, nil, wrapError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
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

func webhookEventID(accountID string, entryTime int64, kind string, index int, raw json.RawMessage) string {
	canonical := raw
	var value any
	if json.Unmarshal(raw, &value) == nil {
		if encoded, err := json.Marshal(value); err == nil {
			canonical = encoded
		}
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(accountID))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(strconv.FormatInt(entryTime, 10)))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(kind))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(strconv.Itoa(index)))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(canonical)
	return hex.EncodeToString(digest.Sum(nil))
}

var _ socialhub.WebhookHandler = (*Client)(nil)
var _ socialhub.ChallengeHandler = (*Client)(nil)
