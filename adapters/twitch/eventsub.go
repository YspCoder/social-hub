package twitch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxWebhookBodyBytes = 1 << 20
	maxEventAge         = 10 * time.Minute
	headerMessageID     = "Twitch-Eventsub-Message-Id"
	headerMessageType   = "Twitch-Eventsub-Message-Type"
	headerSignature     = "Twitch-Eventsub-Message-Signature"
	headerTimestamp     = "Twitch-Eventsub-Message-Timestamp"
	headerSubType       = "Twitch-Eventsub-Subscription-Type"
	headerSubVersion    = "Twitch-Eventsub-Subscription-Version"
)

func (c *Client) CreateWebhookSubscription(ctx context.Context, eventType, version string, condition map[string]string, callback string, options ...socialhub.CallOption) (*EventSubPage, error) {
	if c.webhookSecret == "" {
		return nil, unsupported("eventsub_create", "account.settings.eventsub_secret_ref is required")
	}
	if !validEventToken(eventType) || !validEventToken(version) || len(condition) == 0 {
		return nil, invalidArgument("eventsub_create", "event type, version, and condition are required")
	}
	for key, value := range condition {
		if !validEventToken(key) || strings.TrimSpace(value) == "" {
			return nil, invalidArgument("eventsub_create", "condition keys and values must not be empty")
		}
	}
	if !validWebhookCallback(callback) {
		return nil, invalidArgument("eventsub_create", "callback must be an HTTPS URL on the default port without credentials")
	}
	body := struct {
		Type      string            `json:"type"`
		Version   string            `json:"version"`
		Condition map[string]string `json:"condition"`
		Transport struct {
			Method   string `json:"method"`
			Callback string `json:"callback"`
			Secret   string `json:"secret"`
		} `json:"transport"`
	}{Type: eventType, Version: version, Condition: condition}
	body.Transport.Method, body.Transport.Callback, body.Transport.Secret = "webhook", callback, c.webhookSecret
	var response eventSubAPIPage
	if err := appRequest(ctx, c.appTransport, http.MethodPost, "/eventsub/subscriptions", nil, body, &response, options...); err != nil {
		return nil, err
	}
	return mapEventSubPage(response, "eventsub_create")
}

func (c *Client) ListSubscriptions(ctx context.Context, input EventSubListRequest, options ...socialhub.CallOption) (*EventSubPage, error) {
	if input.Type != "" && input.Status != "" {
		return nil, invalidArgument("eventsub_list", "type and status filters are mutually exclusive")
	}
	query := url.Values{}
	if input.Type != "" {
		if !validEventToken(input.Type) {
			return nil, invalidArgument("eventsub_list", "event type is invalid")
		}
		query.Set("type", input.Type)
	}
	if input.Status != "" {
		if !validEventToken(input.Status) {
			return nil, invalidArgument("eventsub_list", "status is invalid")
		}
		query.Set("status", input.Status)
	}
	if err := setPaging(query, input.Cursor, input.MaxResults); err != nil {
		return nil, err
	}
	var response eventSubAPIPage
	if err := appRequest(ctx, c.appTransport, http.MethodGet, "/eventsub/subscriptions", query, nil, &response, options...); err != nil {
		return nil, err
	}
	return mapEventSubPage(response, "eventsub_list")
}

func (c *Client) DeleteSubscription(ctx context.Context, subscriptionID string, options ...socialhub.CallOption) error {
	if strings.TrimSpace(subscriptionID) == "" {
		return invalidArgument("eventsub_delete", "subscription ID is required")
	}
	return appRequest(ctx, c.appTransport, http.MethodDelete, "/eventsub/subscriptions", url.Values{"id": {subscriptionID}}, nil, nil, options...)
}

func mapEventSubPage(response eventSubAPIPage, operation string) (*EventSubPage, error) {
	for _, item := range response.Data {
		if item.ID == "" || item.Type == "" || item.Version == "" {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return &EventSubPage{
		Items: response.Data, Total: response.Total, TotalCost: response.TotalCost, MaxTotalCost: response.MaxTotalCost,
		NextCursor: stringPointer(response.Pagination.Cursor),
	}, nil
}

func (c *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if c.webhookSecret == "" {
		return unsupported("webhook_verify", "account.settings.eventsub_secret_ref is not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "EventSub webhook must be a bounded, non-empty POST body")
	}
	messageID := request.Header.Get(headerMessageID)
	timestampValue := request.Header.Get(headerTimestamp)
	signatureValue := request.Header.Get(headerSignature)
	if messageID == "" || timestampValue == "" || !strings.HasPrefix(signatureValue, "sha256=") {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	timestamp, err := time.Parse(time.RFC3339Nano, timestampValue)
	if err != nil {
		return invalidArgument("webhook_verify", "EventSub timestamp is invalid")
	}
	now := c.clock.Now()
	if now.Sub(timestamp) > maxEventAge || timestamp.Sub(now) > maxEventAge {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signatureValue, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, err)
	}
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = mac.Write([]byte(messageID))
	_, _ = mac.Write([]byte(timestampValue))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return platformError("webhook_verify", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	return nil
}

func (c *Client) Decode(_ context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if request == nil {
		return nil, invalidArgument("webhook_decode", "request is required")
	}
	messageID, messageType := request.Header.Get(headerMessageID), request.Header.Get(headerMessageType)
	if messageID == "" || messageType == "" {
		return nil, invalidArgument("webhook_decode", "EventSub message ID and type headers are required")
	}
	var envelope struct {
		Subscription EventSubSubscription `json:"subscription"`
		Event        json.RawMessage      `json:"event"`
		Challenge    string               `json:"challenge"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError("webhook_decode", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if envelope.Subscription.ID == "" || envelope.Subscription.Type == "" || envelope.Subscription.Version == "" {
		return nil, invalidArgument("webhook_decode", "EventSub subscription metadata is incomplete")
	}
	if expected := request.Header.Get(headerSubType); expected != "" && expected != envelope.Subscription.Type {
		return nil, invalidArgument("webhook_decode", "subscription type header does not match payload")
	}
	if expected := request.Header.Get(headerSubVersion); expected != "" && expected != envelope.Subscription.Version {
		return nil, invalidArgument("webhook_decode", "subscription version header does not match payload")
	}
	eventType := envelope.Subscription.Type
	switch messageType {
	case "notification":
		if len(envelope.Event) == 0 || bytes.Equal(envelope.Event, []byte("null")) {
			return nil, invalidArgument("webhook_decode", "notification event payload is required")
		}
	case "webhook_callback_verification":
		if envelope.Challenge == "" {
			return nil, invalidArgument("webhook_decode", "verification challenge is required")
		}
		eventType = "eventsub.challenge"
	case "revocation":
		eventType = "eventsub.revocation." + envelope.Subscription.Type
	default:
		return nil, invalidArgument("webhook_decode", "unsupported EventSub message type")
	}
	payload := EventSubPayload{
		Subscription: envelope.Subscription, Event: append(json.RawMessage(nil), envelope.Event...),
		Challenge: envelope.Challenge, Raw: append(json.RawMessage(nil), body...),
	}
	return []socialhub.Event{{
		ID: messageID, Type: eventType, Platform: "twitch", AccountID: c.accountID, Payload: payload,
	}}, nil
}

// HandleChallenge verifies a callback-verification delivery and returns its
// plain-text challenge response.
func (c *Client) HandleChallenge(ctx context.Context, request *http.Request) (int, []byte, error) {
	if request == nil || request.Body == nil {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "request body is required")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxWebhookBodyBytes+1))
	if err != nil || len(body) > maxWebhookBodyBytes {
		return http.StatusBadRequest, nil, platformError("webhook_challenge", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	if err := c.Verify(ctx, request, body); err != nil {
		return http.StatusForbidden, nil, err
	}
	events, err := c.Decode(ctx, request, body)
	if err != nil || len(events) != 1 || events[0].Type != "eventsub.challenge" {
		return http.StatusBadRequest, nil, firstError(err, invalidArgument("webhook_challenge", "delivery is not a callback verification"))
	}
	payload, ok := events[0].Payload.(EventSubPayload)
	if !ok || payload.Challenge == "" {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "challenge payload is invalid")
	}
	return http.StatusOK, []byte(payload.Challenge), nil
}

func validEventSubSecret(value string) bool {
	if len(value) < 10 || len(value) > 100 {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}

func validWebhookCallback(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil && parsed.Fragment == "" && (parsed.Port() == "" || parsed.Port() == "443")
}

func validEventToken(value string) bool {
	if strings.TrimSpace(value) == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character == '.' || character == '_' || character == '-' || (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
			return false
		}
	}
	return true
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}

var _ socialhub.WebhookHandler = (*Client)(nil)
var _ socialhub.ChallengeHandler = (*Client)(nil)
