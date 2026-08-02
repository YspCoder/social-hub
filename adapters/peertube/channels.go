package peertube

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetChannel(ctx context.Context, handle string, options ...socialhub.CallOption) (*VideoChannel, error) {
	if !validActorHandle(handle) {
		return nil, invalidArgument("get_channel", "a valid channel handle is required")
	}
	var response VideoChannel
	if err := c.transport.JSON(ctx, http.MethodGet, "/video-channels/"+url.PathEscape(handle), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID < 1 || response.Name == "" {
		return nil, platformError("get_channel", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) ListChannels(ctx context.Context, input ChannelListRequest, options ...socialhub.CallOption) (socialhub.Page[VideoChannel], error) {
	query, start, limit, err := pageQuery("list_channels", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[VideoChannel]{}, err
	}
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	var response VideoChannelListResponse
	if err := c.transport.JSON(ctx, http.MethodGet, "/video-channels", query, nil, &response, options...); err != nil {
		return socialhub.Page[VideoChannel]{}, err
	}
	next, previous, hasMore, err := pageCursors(response.Total, start, limit, len(response.Data))
	if err != nil {
		return socialhub.Page[VideoChannel]{}, err
	}
	return socialhub.Page[VideoChannel]{Items: response.Data, NextCursor: next, PrevCursor: previous, HasMore: hasMore}, nil
}
