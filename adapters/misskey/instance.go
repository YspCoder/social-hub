package misskey

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (c *Client) Instance(ctx context.Context, options ...socialhub.CallOption) (*InstanceInfo, error) {
	var response misskeyMeta
	if err := c.post(ctx, "meta", struct {
		Detail bool `json:"detail"`
	}{Detail: true}, &response, options...); err != nil {
		return nil, err
	}
	if !validBoundedString(response.Name, 4096) || !validBoundedString(response.Version, 512) ||
		(response.URI != "" && !validBoundedString(response.URI, 512)) ||
		(response.MediaProxy != "" && !validHTTPURL(response.MediaProxy)) {
		return nil, platformError("instance", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &InstanceInfo{
		Name: response.Name, ShortName: response.ShortName, Version: response.Version,
		Description: response.Description, URI: response.URI,
		DisableLocalTimeline: response.DisableLocalTimeline, DisableGlobalTimeline: response.DisableGlobalTimeline,
		MediaProxy: response.MediaProxy,
	}, nil
}
