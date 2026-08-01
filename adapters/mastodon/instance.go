package mastodon

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) Instance(ctx context.Context, options ...socialhub.CallOption) (*InstanceInfo, error) {
	var response mastodonInstance
	if err := c.transport.JSON(ctx, http.MethodGet, "/api/v2/instance", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Domain == "" || response.Version == "" {
		return nil, platformError("instance", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &InstanceInfo{
		Domain: response.Domain, Title: response.Title, Version: response.Version, SourceURL: response.SourceURL,
		MastodonAPIVersion:  response.APIVersions.Mastodon,
		MaxStatusCharacters: response.Configuration.Statuses.MaxCharacters,
		MaxMediaAttachments: response.Configuration.Statuses.MaxMediaAttachments,
		ImageSizeLimit:      response.Configuration.MediaAttachments.ImageSizeLimit,
		VideoSizeLimit:      response.Configuration.MediaAttachments.VideoSizeLimit,
	}, nil
}
