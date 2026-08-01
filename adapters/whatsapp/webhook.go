package whatsapp

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

type webhookEnvelope struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Field string          `json:"field"`
			Value json.RawMessage `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

type webhookValue struct {
	MessagingProduct string            `json:"messaging_product"`
	Metadata         WebhookMetadata   `json:"metadata"`
	Contacts         []contactWire     `json:"contacts"`
	Messages         []json.RawMessage `json:"messages"`
	Statuses         []json.RawMessage `json:"statuses"`
}

type contactWire struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WAID string `json:"wa_id"`
}

type messageWire struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Context   *struct {
		ID string `json:"id"`
	} `json:"context"`
}

type statusWire struct {
	ID           string          `json:"id"`
	Status       string          `json:"status"`
	Timestamp    string          `json:"timestamp"`
	RecipientID  string          `json:"recipient_id"`
	Conversation json.RawMessage `json:"conversation"`
	Pricing      json.RawMessage `json:"pricing"`
	Errors       json.RawMessage `json:"errors"`
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.appSecret == "" {
		return unsupported("webhook_verify", "account.settings.app_secret_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "WhatsApp webhook must be a bounded, non-empty POST body")
	}
	signature := request.Header.Get("X-Hub-Signature-256")
	if !strings.HasPrefix(signature, "sha256=") {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.appSecret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	if len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return nil, invalidArgument("webhook_decode", "WhatsApp webhook body must be bounded and non-empty")
	}
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.Object != "whatsapp_business_account" || len(envelope.Entry) == 0 {
		return nil, invalidArgument("webhook_decode", "webhook object must be whatsapp_business_account with entries")
	}
	var events []socialhub.Event
	for _, entry := range envelope.Entry {
		if entry.ID == "" {
			return nil, invalidArgument("webhook_decode", "business account entry ID is required")
		}
		if c.businessID != "" && entry.ID != c.businessID {
			return nil, invalidArgument("webhook_decode", "webhook business account ID does not match the configured account")
		}
		for index, change := range entry.Changes {
			if strings.TrimSpace(change.Field) == "" || len(change.Value) == 0 {
				return nil, invalidArgument("webhook_decode", "change field and value are required")
			}
			if change.Field != "messages" {
				continue
			}
			var value webhookValue
			if err := json.Unmarshal(change.Value, &value); err != nil {
				return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
			if value.MessagingProduct != "whatsapp" || value.Metadata.PhoneNumberID != c.phoneNumberID {
				return nil, invalidArgument("webhook_decode", "webhook phone_number_id does not match the configured account")
			}
			contacts := make(map[string]WebhookContact, len(value.Contacts))
			for _, contact := range value.Contacts {
				contacts[contact.WAID] = WebhookContact{WAID: contact.WAID, Name: contact.Profile.Name}
			}
			for _, raw := range value.Messages {
				var message messageWire
				if err := json.Unmarshal(raw, &message); err != nil || message.ID == "" || message.Type == "" {
					return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
				}
				payload := MessageWebhookPayload{
					BusinessAccountID: entry.ID, Metadata: value.Metadata, From: message.From, ID: message.ID,
					Timestamp: unixTimePointer(message.Timestamp), Type: message.Type, Raw: append(json.RawMessage(nil), raw...),
				}
				if message.Context != nil {
					payload.ReplyToID = message.Context.ID
				}
				if contact, found := contacts[message.From]; found {
					copy := contact
					payload.Contact = &copy
				}
				events = append(events, socialhub.Event{
					ID: message.ID, Type: "whatsapp.message." + message.Type, Platform: "whatsapp", AccountID: c.accountID, Payload: payload,
				})
			}
			for _, raw := range value.Statuses {
				var status statusWire
				if err := json.Unmarshal(raw, &status); err != nil || status.ID == "" || status.Status == "" {
					return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
				}
				payload := StatusWebhookPayload{
					BusinessAccountID: entry.ID, Metadata: value.Metadata, ID: status.ID, Status: status.Status,
					RecipientID: status.RecipientID, Timestamp: unixTimePointer(status.Timestamp),
					Conversation: append(json.RawMessage(nil), status.Conversation...), Pricing: append(json.RawMessage(nil), status.Pricing...),
					Errors: append(json.RawMessage(nil), status.Errors...), Raw: append(json.RawMessage(nil), raw...),
				}
				events = append(events, socialhub.Event{
					ID: statusEventID(status), Type: "whatsapp.status." + status.Status,
					Platform: "whatsapp", AccountID: c.accountID, Payload: payload,
				})
			}
			if len(value.Messages) == 0 && len(value.Statuses) == 0 {
				events = append(events, socialhub.Event{
					ID: changeEventID(entry.ID, change.Field, index, change.Value), Type: "whatsapp.change." + change.Field,
					Platform: "whatsapp", AccountID: c.accountID, Payload: append(json.RawMessage(nil), change.Value...),
				})
			}
		}
	}
	return events, nil
}

// HandleChallenge validates Meta's GET subscription challenge.
func (c *Client) HandleChallenge(_ context.Context, request *http.Request) (int, []byte, error) {
	if request == nil || request.Method != http.MethodGet || c.verifyToken == "" {
		return http.StatusForbidden, nil, platformError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	query := request.URL.Query()
	provided := query.Get("hub.verify_token")
	challenge := query.Get("hub.challenge")
	if query.Get("hub.mode") != "subscribe" || challenge == "" || len(provided) != len(c.verifyToken) || !hmac.Equal([]byte(provided), []byte(c.verifyToken)) {
		return http.StatusForbidden, nil, platformError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return http.StatusOK, []byte(challenge), nil
}

func unixTimePointer(value string) *time.Time {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds < 0 {
		return nil
	}
	parsed := time.Unix(seconds, 0).UTC()
	return &parsed
}

func statusEventID(status statusWire) string {
	return status.ID + ":" + status.Status + ":" + status.Timestamp
}

func changeEventID(entryID, field string, index int, payload []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(entryID))
	_, _ = digest.Write([]byte(field))
	_, _ = digest.Write([]byte(strconv.Itoa(index)))
	_, _ = digest.Write(payload)
	return hex.EncodeToString(digest.Sum(nil))
}

var _ socialhub.WebhookHandler = (*Client)(nil)
var _ socialhub.ChallengeHandler = (*Client)(nil)
