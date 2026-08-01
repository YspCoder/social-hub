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

	"social-hub/pkg/socialhub"
)

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookSecret == "" {
		return unsupported("webhook_verify", "webhook.secret_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost {
		return invalidArgument("webhook_verify", "Instagram webhook deliveries must use POST")
	}
	header := request.Header.Get("X-Hub-Signature-256")
	if !strings.HasPrefix(header, "sha256=") {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	var envelope webhookEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.Object != "instagram" {
		return nil, invalidArgument("webhook_decode", "webhook object is not instagram")
	}
	var events []socialhub.Event
	for _, entry := range envelope.Entry {
		for index, change := range entry.Changes {
			payload, _ := json.Marshal(change)
			events = append(events, socialhub.Event{
				ID: webhookEventID(entry.ID, entry.Time, index, payload), Type: "instagram." + change.Field,
				Platform: "instagram", AccountID: c.accountID, Payload: json.RawMessage(payload),
			})
		}
	}
	return events, nil
}

// HandleChallenge validates Meta's webhook subscription challenge.
func (c *Client) HandleChallenge(_ context.Context, request *http.Request) (int, []byte, error) {
	if request == nil {
		return http.StatusForbidden, nil, wrapError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	query := request.URL.Query()
	if query.Get("hub.mode") != "subscribe" || c.webhookToken == "" || !hmac.Equal([]byte(query.Get("hub.verify_token")), []byte(c.webhookToken)) {
		return http.StatusForbidden, nil, wrapError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return http.StatusOK, []byte(query.Get("hub.challenge")), nil
}

type webhookEnvelope struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Time    int64  `json:"time"`
		Changes []struct {
			Field string          `json:"field"`
			Value json.RawMessage `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

func webhookEventID(entryID string, timestamp int64, index int, payload []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(entryID))
	_, _ = digest.Write([]byte(strconv.FormatInt(timestamp, 10)))
	_, _ = digest.Write([]byte(strconv.Itoa(index)))
	_, _ = digest.Write(payload)
	return hex.EncodeToString(digest.Sum(nil))
}

var _ socialhub.WebhookHandler = (*Client)(nil)
var _ socialhub.ChallengeHandler = (*Client)(nil)
