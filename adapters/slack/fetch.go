package slack

import (
	"context"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type historyRequest struct {
	Channel   string `json:"channel"`
	Cursor    string `json:"cursor,omitempty"`
	Inclusive bool   `json:"inclusive,omitempty"`
	Latest    string `json:"latest,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Oldest    string `json:"oldest,omitempty"`
	TS        string `json:"ts,omitempty"`
}

type historyResponse struct {
	Messages []wireMessage `json:"messages"`
	HasMore  bool          `json:"has_more"`
	Metadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if err := c.requireScopes("users.info", "users:read"); err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if !validSlackID(userID, "UWB") {
		return nil, invalidArgument("users.info", "user ID must be a Slack user or bot ID")
	}
	var response struct {
		User wireUser `json:"user"`
	}
	if err := c.call(ctx, "users.info", struct {
		User string `json:"user"`
	}{User: userID}, &response, options...); err != nil {
		return nil, err
	}
	if response.User.ID != userID {
		return nil, platformError("users.info", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapUser(c.accountID, response.User), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	channelID, timestamp, err := parseCompositeID(postID, "conversations.history")
	if err != nil {
		return nil, err
	}
	if err := c.requireHistoryScope("conversations.history", channelID); err != nil {
		return nil, err
	}
	var response historyResponse
	if err := c.call(ctx, "conversations.history", historyRequest{
		Channel: channelID, Inclusive: true, Oldest: timestamp, Latest: timestamp, Limit: 1,
	}, &response, options...); err != nil {
		return nil, err
	}
	for _, message := range response.Messages {
		if messageTimestamp(message) == timestamp {
			return mapPost(c.accountID, channelID, message, c.clock.Now()), nil
		}
	}
	return nil, platformError("conversations.history", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	channelID := strings.TrimSpace(input.UserID)
	if channelID == "" {
		channelID = c.defaultChannelID
	}
	if !validSlackID(channelID, "CGD") {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("conversations.history", "user_id or default_channel_id must identify a Slack conversation")
	}
	if err := c.requireHistoryScope("conversations.history", channelID); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	limit, err := slackPageLimit(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if !validCursor(input.Cursor) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("conversations.history", "cursor must be a bounded opaque Slack cursor")
	}
	request := historyRequest{Channel: channelID, Cursor: input.Cursor, Limit: limit}
	if input.StartTime != nil {
		request.Oldest = slackTime(*input.StartTime)
	}
	if input.EndTime != nil {
		if input.StartTime != nil && input.EndTime.Before(*input.StartTime) {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("conversations.history", "end_time must not precede start_time")
		}
		request.Latest = slackTime(*input.EndTime)
	}
	var response historyResponse
	if err := c.call(ctx, "conversations.history", request, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Messages))
	observedAt := c.clock.Now()
	for _, message := range response.Messages {
		if validTimestamp(messageTimestamp(message)) {
			items = append(items, *mapPost(c.accountID, channelID, message, observedAt))
		}
	}
	return slackPage(items, response.Metadata.NextCursor, response.HasMore), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	channelID, rootTS, err := parseCompositeID(input.PostID, "conversations.replies")
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if err := c.requireHistoryScope("conversations.replies", channelID); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	limit, err := slackPageLimit(input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if !validCursor(input.Cursor) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("conversations.replies", "cursor must be a bounded opaque Slack cursor")
	}
	var response historyResponse
	if err := c.call(ctx, "conversations.replies", historyRequest{
		Channel: channelID, TS: rootTS, Cursor: input.Cursor, Limit: limit,
	}, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Messages))
	observedAt := c.clock.Now()
	for _, message := range response.Messages {
		if messageTimestamp(message) == rootTS || !validTimestamp(messageTimestamp(message)) {
			continue
		}
		items = append(items, mapComment(c.accountID, channelID, rootTS, message, observedAt))
	}
	return slackPage(items, response.Metadata.NextCursor, response.HasMore), nil
}

func slackPageLimit(requested int) (int, error) {
	if requested < 0 {
		return 0, invalidArgument("pagination", "max_results must not be negative")
	}
	if requested == 0 {
		return 15, nil
	}
	if requested > 1000 {
		return 1000, nil
	}
	return requested, nil
}

func validCursor(value string) bool {
	if len(value) > 1024 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func slackPage[T any](items []T, cursor string, hasMore bool) socialhub.Page[T] {
	var next *string
	if strings.TrimSpace(cursor) != "" {
		next = stringPointer(cursor)
	}
	return socialhub.Page[T]{Items: items, NextCursor: next, HasMore: hasMore || next != nil}
}

func slackTime(value time.Time) string {
	return strconv.FormatInt(value.UTC().Unix(), 10) + "." + strconv.FormatInt(int64(value.UTC().Nanosecond()/1000)+1000000, 10)[1:]
}
