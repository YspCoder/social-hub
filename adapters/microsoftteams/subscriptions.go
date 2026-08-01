package microsoftteams

import (
	"context"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type subscriptionInput struct {
	ChangeType               string    `json:"changeType,omitempty"`
	NotificationURL          string    `json:"notificationUrl,omitempty"`
	LifecycleNotificationURL string    `json:"lifecycleNotificationUrl,omitempty"`
	Resource                 string    `json:"resource,omitempty"`
	ExpirationDateTime       time.Time `json:"expirationDateTime"`
	ClientState              string    `json:"clientState,omitempty"`
	IncludeResourceData      bool      `json:"includeResourceData"`
}

func (c *Client) CreateSubscription(ctx context.Context, input CreateSubscriptionRequest, options ...socialhub.CallOption) (*Subscription, error) {
	const operation = "create_subscription"
	if c.clientState == "" {
		return nil, unsupported(operation, "webhook.secret_ref must supply the subscription clientState")
	}
	changeType, err := validateSubscriptionInput(operation, input, c.clock.Now())
	if err != nil {
		return nil, err
	}
	request := subscriptionInput{
		ChangeType: changeType, NotificationURL: input.NotificationURL,
		LifecycleNotificationURL: input.LifecycleNotificationURL, Resource: input.Resource,
		ExpirationDateTime: input.ExpirationDateTime.UTC(), ClientState: c.clientState, IncludeResourceData: false,
	}
	var subscription Subscription
	if err := c.post(ctx, operation, "subscriptions", request, &subscription, options...); err != nil {
		return nil, err
	}
	if !validOpaqueID(subscription.ID, 2048) {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &subscription, nil
}

func (c *Client) RenewSubscription(ctx context.Context, id string, expiration time.Time, options ...socialhub.CallOption) (*Subscription, error) {
	const operation = "renew_subscription"
	if !validOpaqueID(id, 2048) {
		return nil, invalidArgument(operation, "a valid subscription ID is required")
	}
	if !expiration.After(c.clock.Now()) {
		return nil, invalidArgument(operation, "expiration must be in the future")
	}
	var subscription Subscription
	input := struct {
		ExpirationDateTime time.Time `json:"expirationDateTime"`
	}{ExpirationDateTime: expiration.UTC()}
	if err := c.patch(ctx, operation, "subscriptions/"+id, input, &subscription, options...); err != nil {
		return nil, err
	}
	if !validOpaqueID(subscription.ID, 2048) {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &subscription, nil
}

func (c *Client) DeleteSubscription(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if !validOpaqueID(id, 2048) {
		return invalidArgument("delete_subscription", "a valid subscription ID is required")
	}
	return c.delete(ctx, "delete_subscription", "subscriptions/"+id, options...)
}

func validateSubscriptionInput(operation string, input CreateSubscriptionRequest, now time.Time) (string, error) {
	if !validResource(input.Resource) {
		return "", invalidArgument(operation, "resource must be a relative Microsoft Graph resource path")
	}
	if !validHTTPSURL(input.NotificationURL) {
		return "", invalidArgument(operation, "notification_url must be an absolute HTTPS URL without credentials or fragment")
	}
	if input.LifecycleNotificationURL != "" && !validHTTPSURL(input.LifecycleNotificationURL) {
		return "", invalidArgument(operation, "lifecycle_notification_url must be an absolute HTTPS URL without credentials or fragment")
	}
	if !input.ExpirationDateTime.After(now) {
		return "", invalidArgument(operation, "expiration must be in the future")
	}
	if input.ExpirationDateTime.Sub(now) > time.Hour && input.LifecycleNotificationURL == "" {
		return "", invalidArgument(operation, "Teams subscriptions longer than one hour require lifecycle_notification_url")
	}
	if len(input.ChangeTypes) == 0 {
		return "", invalidArgument(operation, "at least one change type is required")
	}
	seen := make(map[string]struct{}, len(input.ChangeTypes))
	values := make([]string, 0, len(input.ChangeTypes))
	for _, value := range input.ChangeTypes {
		value = strings.TrimSpace(value)
		if value != "created" && value != "updated" && value != "deleted" {
			return "", invalidArgument(operation, "change types must be created, updated, or deleted")
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	return strings.Join(values, ","), nil
}

func validResource(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > 4096 || strings.Contains(value, "://") || strings.ContainsAny(value, "\r\n\\#") {
		return false
	}
	path := strings.ToLower(strings.SplitN(value, "?", 2)[0])
	if strings.HasPrefix(path, "/teams/") || strings.HasPrefix(path, "/chats/") {
		return strings.Contains(path, "messages")
	}
	return strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/chats/getallmessages")
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}
