package strava

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"social-hub/pkg/socialhub"
)

const (
	maxWebhookBodyBytes = 1 << 20
	maxChallengeBytes   = 4096
)

func (client *Client) Verify(_ context.Context, request *http.Request, body []byte) error {
	if client.subscriptionID <= 0 || client.verifyToken == "" {
		return unsupported("webhook_verify", "subscription_id and webhook.token_ref are not configured")
	}
	if request == nil || request.Method != http.MethodPost || len(body) == 0 || len(body) > maxWebhookBodyBytes {
		return invalidArgument("webhook_verify", "Strava webhook must be a bounded, non-empty POST body")
	}
	_, err := client.parseWebhook("webhook_verify", body)
	return err
}

func (client *Client) Decode(ctx context.Context, request *http.Request, body []byte) ([]socialhub.Event, error) {
	if err := client.Verify(ctx, request, body); err != nil {
		return nil, err
	}
	payload, err := client.parseWebhook("webhook_decode", body)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(body)
	return []socialhub.Event{{
		ID: hex.EncodeToString(digest[:]), Type: "strava." + payload.ObjectType + "." + payload.AspectType,
		Platform: "strava", AccountID: client.accountID, Payload: payload,
	}}, nil
}

func (client *Client) parseWebhook(operation string, body []byte) (WebhookEvent, error) {
	var payload WebhookEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if (payload.ObjectType != "activity" && payload.ObjectType != "athlete") ||
		(payload.AspectType != "create" && payload.AspectType != "update" && payload.AspectType != "delete") ||
		payload.ObjectID <= 0 || payload.OwnerID <= 0 || payload.EventTime <= 0 || payload.SubscriptionID <= 0 {
		return WebhookEvent{}, invalidArgument(operation, "Strava webhook object, aspect, IDs, and event time are invalid")
	}
	athleteID, err := strconv.ParseInt(client.athleteID, 10, 64)
	if err != nil || payload.OwnerID != athleteID || payload.SubscriptionID != client.subscriptionID {
		return WebhookEvent{}, platformError(operation, socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	payload.Raw = append(json.RawMessage(nil), body...)
	return payload, nil
}

// HandleChallenge validates Strava's application subscription handshake and
// returns the JSON object that an HTTP layer must echo.
func (client *Client) HandleChallenge(_ context.Context, request *http.Request) (int, []byte, error) {
	if client.subscriptionID <= 0 || client.verifyToken == "" {
		return http.StatusBadRequest, nil, unsupported("webhook_challenge", "subscription_id and webhook.token_ref are not configured")
	}
	if request == nil || request.Method != http.MethodGet {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "Strava webhook challenge must use GET")
	}
	query := request.URL.Query()
	if query.Get("hub.mode") != "subscribe" || !hmac.Equal([]byte(query.Get("hub.verify_token")), []byte(client.verifyToken)) {
		return http.StatusForbidden, nil, platformError("webhook_challenge", socialhub.CodePermissionDenied, socialhub.ClassPermanent, nil)
	}
	challenge := query.Get("hub.challenge")
	if !validOpaque(challenge, maxChallengeBytes) {
		return http.StatusBadRequest, nil, invalidArgument("webhook_challenge", "hub.challenge must be non-empty and bounded")
	}
	body, err := json.Marshal(map[string]string{"hub.challenge": challenge})
	if err != nil {
		return http.StatusInternalServerError, nil, platformError("webhook_challenge", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return http.StatusOK, body, nil
}

var _ socialhub.WebhookHandler = (*Client)(nil)
var _ socialhub.ChallengeHandler = (*Client)(nil)
