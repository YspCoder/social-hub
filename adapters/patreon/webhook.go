package patreon

import (
	"context"
	"crypto/hmac"
	"crypto/md5" // Patreon mandates HMAC-MD5 for webhook signatures.
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxWebhookBodyBytes = 8 << 20

// WebhookPayload preserves one signed Patreon JSON:API event.
type WebhookPayload struct {
	Trigger  string          `json:"trigger"`
	DataType string          `json:"data_type"`
	DataID   string          `json:"data_id"`
	Raw      json.RawMessage `json:"raw"`
}

func (client *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if client.webhookSecret == "" {
		return unsupported("webhook_verify", "Patreon webhook secret is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Patreon webhook must be a bounded, non-empty POST body")
	}
	provided, err := hex.DecodeString(strings.TrimSpace(request.Header.Get("X-Patreon-Signature")))
	if err != nil || len(provided) != md5.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(md5.New, []byte(client.webhookSecret))
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
	trigger := strings.TrimSpace(request.Header.Get("X-Patreon-Event"))
	if !validWebhookTrigger(trigger) {
		return nil, invalidArgument("webhook_decode", "X-Patreon-Event trigger is invalid")
	}
	var envelope struct {
		Data resourceIdentifier `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if !validResourceID(envelope.Data.ID) || !validResourceID(envelope.Data.Type) {
		return nil, invalidArgument("webhook_decode", "Patreon webhook data type and ID are required")
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(trigger))
	_, _ = digest.Write(body)
	payload := WebhookPayload{
		Trigger: trigger, DataType: envelope.Data.Type, DataID: envelope.Data.ID,
		Raw: append(json.RawMessage(nil), body...),
	}
	return []socialhub.Event{{
		ID: hex.EncodeToString(digest.Sum(nil)), Type: trigger, Platform: "patreon", AccountID: client.accountID, Payload: payload,
	}}, nil
}

func validWebhookTrigger(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || strings.ContainsRune(":._-", character) {
			continue
		}
		return false
	}
	return true
}
