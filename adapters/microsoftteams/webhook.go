package microsoftteams

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

type notificationEnvelope struct {
	Value            []json.RawMessage `json:"value"`
	ValidationTokens []string          `json:"validationTokens,omitempty"`
}

type notificationWire struct {
	Notification
	EncryptedContent json.RawMessage `json:"encryptedContent,omitempty"`
}

// HandleChallenge returns the URL-decoded validation token that an HTTP layer must echo as text/plain.
func (c *Client) HandleChallenge(request *http.Request) (string, error) {
	if request == nil || request.Method != http.MethodPost {
		return "", invalidArgument("webhook_challenge", "Graph webhook validation must use POST")
	}
	token := request.URL.Query().Get("validationToken")
	if strings.TrimSpace(token) == "" || len(token) > 4096 {
		return "", invalidArgument("webhook_challenge", "validationToken is required and must be bounded")
	}
	return token, nil
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.clientState == "" {
		return unsupported("webhook_verify", "webhook.secret_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost {
		return invalidArgument("webhook_verify", "Microsoft Graph webhooks must use POST")
	}
	if request.URL.Query().Has("validationToken") {
		_, err := c.HandleChallenge(request)
		return err
	}
	_, err := c.decodeNotifications(body, "webhook_verify")
	return err
}

func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if c.clientState == "" {
		return nil, unsupported("webhook_decode", "webhook.secret_ref is not configured")
	}
	if request != nil && request.URL.Query().Has("validationToken") {
		token, err := c.HandleChallenge(request)
		if err != nil {
			return nil, err
		}
		return []socialhub.Event{{
			ID: "validation", Type: "microsoftteams.validation", Platform: "microsoft-teams", AccountID: c.accountID,
			Payload: token,
		}}, nil
	}
	notifications, err := c.decodeNotifications(body, "webhook_decode")
	if err != nil {
		return nil, err
	}
	events := make([]socialhub.Event, 0, len(notifications))
	for _, notification := range notifications {
		events = append(events, socialhub.Event{
			ID: notificationEventID(notification), Type: "microsoftteams." + notification.ChangeType,
			Platform: "microsoft-teams", AccountID: c.accountID, Payload: notification,
		})
	}
	return events, nil
}

func (c *Client) decodeNotifications(body []byte, operation string) ([]Notification, error) {
	if c.clientState == "" {
		return nil, unsupported(operation, "webhook.secret_ref is not configured")
	}
	var envelope notificationEnvelope
	if len(body) == 0 || json.Unmarshal(body, &envelope) != nil {
		return nil, invalidArgument(operation, "a valid notification envelope is required")
	}
	if len(envelope.ValidationTokens) > 0 {
		return nil, unsupported(operation, "rich notifications with validationTokens are not supported")
	}
	if len(envelope.Value) == 0 {
		return nil, invalidArgument(operation, "notification envelope value must not be empty")
	}
	notifications := make([]Notification, 0, len(envelope.Value))
	for _, raw := range envelope.Value {
		var wire notificationWire
		if json.Unmarshal(raw, &wire) != nil {
			return nil, invalidArgument(operation, "notification value is malformed")
		}
		if len(wire.EncryptedContent) > 0 && string(wire.EncryptedContent) != "null" {
			return nil, unsupported(operation, "encrypted resource data requires certificate validation and decryption")
		}
		if (wire.ID != "" && !validOpaqueID(wire.ID, 2048)) || !validOpaqueID(wire.SubscriptionID, 2048) || strings.TrimSpace(wire.ChangeType) == "" || strings.TrimSpace(wire.Resource) == "" {
			return nil, invalidArgument(operation, "notification subscription, change type, and resource are required")
		}
		if len(wire.ClientState) != len(c.clientState) || subtle.ConstantTimeCompare([]byte(wire.ClientState), []byte(c.clientState)) != 1 {
			return nil, platformError(operation, socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
		}
		if c.tenantID != "" && wire.TenantID != c.tenantID {
			return nil, platformError(operation, socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
		}
		wire.Notification.Raw = append(json.RawMessage(nil), raw...)
		notifications = append(notifications, wire.Notification)
	}
	return notifications, nil
}

func notificationEventID(notification Notification) string {
	if notification.ID != "" {
		return notification.ID
	}
	expiration := ""
	if notification.SubscriptionExpirationDateTime != nil {
		expiration = notification.SubscriptionExpirationDateTime.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	sum := sha256.Sum256([]byte(notification.SubscriptionID + "\x00" + notification.ChangeType + "\x00" + notification.Resource + "\x00" + expiration))
	return "graph:" + hex.EncodeToString(sum[:])
}
