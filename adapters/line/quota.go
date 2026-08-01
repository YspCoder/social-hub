package line

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetMessageQuota(ctx context.Context, options ...socialhub.CallOption) (*MessageQuota, error) {
	var quota MessageQuota
	if err := c.request(ctx, c.api, http.MethodGet, "/v2/bot/message/quota", nil, nil, &quota, false, options...); err != nil {
		return nil, err
	}
	if quota.Type != "none" && quota.Type != "limited" {
		return nil, platformError("get_message_quota", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if quota.Type == "limited" && (quota.Value == nil || *quota.Value < 0) {
		return nil, platformError("get_message_quota", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &quota, nil
}

func (c *Client) GetQuotaConsumption(ctx context.Context, options ...socialhub.CallOption) (*QuotaConsumption, error) {
	var consumption QuotaConsumption
	if err := c.request(ctx, c.api, http.MethodGet, "/v2/bot/message/quota/consumption", nil, nil, &consumption, false, options...); err != nil {
		return nil, err
	}
	if consumption.TotalUsage < 0 {
		return nil, platformError("get_quota_consumption", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &consumption, nil
}
