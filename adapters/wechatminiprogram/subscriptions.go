package wechatminiprogram

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

// Send delivers one previously authorized subscription message. The caller
// must obtain and track the user's matching subscription grant.
func (client *Client) Send(ctx context.Context, input SubscriptionMessage, options ...socialhub.CallOption) error {
	const operation = "send_subscription_message"
	normalized, err := normalizeSubscription(input)
	if err != nil {
		return err
	}
	accessToken, err := client.accessToken(ctx, options...)
	if err != nil {
		return err
	}
	query := url.Values{"access_token": {accessToken}}
	return client.doJSON(ctx, operation, http.MethodPost, "/cgi-bin/message/subscribe/send", query, normalized, nil, options...)
}
