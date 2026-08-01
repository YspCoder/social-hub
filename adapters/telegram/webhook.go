package telegram

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-telegram/bot/models"

	"social-hub/pkg/socialhub"
)

// UpdatePayload preserves the maintained typed model and the complete wire
// payload so newly added Bot API fields remain available to callers.
type UpdatePayload struct {
	Update models.Update
	Raw    json.RawMessage
}

func (c *Client) Verify(_ context.Context, request *http.Request, _ []byte) error {
	if c.webhookSecret == "" {
		return unsupported("webhook_verify", "webhook.secret_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost {
		return invalidArgument("webhook_verify", "Telegram webhooks must use POST")
	}
	provided := request.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if len(provided) != len(c.webhookSecret) || subtle.ConstantTimeCompare([]byte(provided), []byte(c.webhookSecret)) != 1 {
		return wrapError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, _ *http.Request, body []byte) ([]socialhub.Event, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	updateIDJSON, ok := fields["update_id"]
	if !ok {
		return nil, invalidArgument("webhook_decode", "update_id is required")
	}
	var updateID int64
	if err := json.Unmarshal(updateIDJSON, &updateID); err != nil {
		return nil, invalidArgument("webhook_decode", "update_id must be an integer")
	}
	eventType, err := updateEventType(fields)
	if err != nil {
		return nil, err
	}
	var update models.Update
	if err := json.Unmarshal(body, &update); err != nil {
		return nil, wrapError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	return []socialhub.Event{{
		ID: strconv.FormatInt(updateID, 10), Type: eventType, Platform: "telegram", AccountID: c.accountID,
		Payload: UpdatePayload{Update: update, Raw: append(json.RawMessage(nil), body...)},
	}}, nil
}

func updateEventType(fields map[string]json.RawMessage) (string, error) {
	var eventType string
	for name := range fields {
		if name == "update_id" {
			continue
		}
		if eventType != "" {
			return "", invalidArgument("webhook_decode", "an update must contain at most one event field")
		}
		eventType = name
	}
	if eventType == "" {
		return "update", nil
	}
	return eventType, nil
}
