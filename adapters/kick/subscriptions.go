package kick

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

var supportedEventVersions = map[string]int{
	"chat.message.sent":                 1,
	"channel.followed":                  1,
	"channel.subscription.renewal":      1,
	"channel.subscription.gifts":        1,
	"channel.subscription.new":          1,
	"channel.reward.redemption.updated": 1,
	"livestream.status.updated":         1,
	"livestream.metadata.updated":       1,
	"moderation.banned":                 1,
	"kicks.gifted":                      1,
}

func (client *Client) ListSubscriptions(ctx context.Context, broadcasterUserID string, options ...socialhub.CallOption) ([]Subscription, error) {
	query := make(url.Values)
	if broadcasterUserID != "" {
		if !validPositiveID(broadcasterUserID) {
			return nil, invalidArgument("list_subscriptions", "broadcaster user ID must be a positive decimal integer")
		}
		query.Set("broadcaster_user_id", broadcasterUserID)
	}
	var response responseEnvelope[[]Subscription]
	if err := client.request(ctx, http.MethodGet, "/public/v1/events/subscriptions", query, nil, &response, options...); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *Client) CreateSubscriptions(ctx context.Context, input CreateSubscriptionsRequest, options ...socialhub.CallOption) ([]SubscriptionResult, error) {
	if len(input.Events) == 0 {
		return nil, invalidArgument("create_subscriptions", "at least one event is required")
	}
	if client.tokenType == "user" {
		if err := client.requireScope("create_subscriptions", "events:subscribe"); err != nil {
			return nil, err
		}
	}
	broadcasterID := input.BroadcasterUserID
	if broadcasterID == "" {
		broadcasterID = client.broadcasterUserID
	}
	if client.tokenType == "app" && !validPositiveID(broadcasterID) {
		return nil, invalidArgument("create_subscriptions", "broadcaster_user_id is required for app access tokens")
	}
	seen := make(map[string]struct{}, len(input.Events))
	for _, event := range input.Events {
		version, supported := supportedEventVersions[event.Name]
		if !supported || event.Version != version {
			return nil, invalidArgument("create_subscriptions", "event name/version is not a documented Kick v1 event")
		}
		key := event.Name + ":" + strconv.Itoa(event.Version)
		if _, exists := seen[key]; exists {
			return nil, invalidArgument("create_subscriptions", "duplicate event subscriptions are not allowed in one request")
		}
		seen[key] = struct{}{}
	}
	body := struct {
		BroadcasterUserID int64                      `json:"broadcaster_user_id,omitempty"`
		Events            []EventSubscriptionRequest `json:"events"`
		Method            string                     `json:"method"`
	}{Events: input.Events, Method: "webhook"}
	if client.tokenType == "app" {
		body.BroadcasterUserID = positiveInt64(broadcasterID)
	}
	var response responseEnvelope[[]SubscriptionResult]
	if err := client.request(ctx, http.MethodPost, "/public/v1/events/subscriptions", nil, body, &response, options...); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *Client) DeleteSubscriptions(ctx context.Context, subscriptionIDs []string, options ...socialhub.CallOption) error {
	if len(subscriptionIDs) == 0 {
		return invalidArgument("delete_subscriptions", "at least one subscription ID is required")
	}
	if client.tokenType == "user" {
		if err := client.requireScope("delete_subscriptions", "events:subscribe"); err != nil {
			return err
		}
	}
	query := make(url.Values)
	for _, id := range subscriptionIDs {
		if !validOpaque(id, 512) {
			return invalidArgument("delete_subscriptions", "subscription IDs must be bounded non-empty values")
		}
		query.Add("id", id)
	}
	return client.request(ctx, http.MethodDelete, "/public/v1/events/subscriptions", query, nil, nil, options...)
}

func (client *Client) FetchWebhookPublicKey(ctx context.Context, options ...socialhub.CallOption) (string, error) {
	var response responseEnvelope[struct {
		PublicKey string `json:"public_key"`
	}]
	if err := client.request(ctx, http.MethodGet, "/public/v1/public-key", nil, nil, &response, options...); err != nil {
		return "", err
	}
	if _, err := parseWebhookPublicKey(response.Data.PublicKey); err != nil {
		return "", platformError("fetch_webhook_public_key", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return response.Data.PublicKey, nil
}

var _ SubscriptionWorkflow = (*Client)(nil)
