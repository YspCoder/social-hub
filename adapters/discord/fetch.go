package discord

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if !validSnowflake(userID) {
		return nil, invalidArgument("get_user", "user ID must be a Discord snowflake")
	}
	var response discordUser
	if err := c.get(ctx, "/users/"+userID, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, wrapError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := c.mapUser(response)
	return &result, nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	channelID, messageID, err := parseMessageID("get_post", postID, c.defaultChannelID)
	if err != nil {
		return nil, err
	}
	message, err := c.getMessage(ctx, channelID, messageID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapPost(*message), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if c.defaultChannelID == "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "account.settings.default_channel_id is required")
	}
	if input.UserID != "" {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Discord history is channel-based; user filtering is not supported by this endpoint")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Discord channel history does not accept time-range filters")
	}
	limit := input.MaxResults
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}}
	if input.Cursor != "" {
		channelID, messageID, err := parseMessageID("list_posts", input.Cursor, c.defaultChannelID)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		if channelID != c.defaultChannelID {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "cursor must belong to the configured default channel")
		}
		query.Set("before", messageID)
	}
	var response []discordMessage
	if err := c.get(ctx, "/channels/"+c.defaultChannelID+"/messages", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response))
	for _, message := range response {
		items = append(items, *c.mapPost(message))
	}
	var next *string
	if len(response) == limit && len(response) > 0 {
		value := composeMessageID(c.defaultChannelID, response[len(response)-1].ID)
		next = &value
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Discord REST API does not expose a portable list-replies endpoint without guild search context")
}
