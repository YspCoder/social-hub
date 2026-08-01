package youtube

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" {
		userID = c.channelID
	}
	if userID != c.channelID {
		return nil, invalidArgument("get_user", "user ID must be the configured YouTube channel ID")
	}
	if err := c.requireScope("get_user", "https://www.googleapis.com/auth/youtube.readonly"); err != nil {
		return nil, err
	}
	query := url.Values{"part": {"snippet,statistics"}, "id": {c.channelID}}
	var response channelList
	if err := c.transport.JSON(ctx, http.MethodGet, "/channels", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Items) != 1 || response.Items[0].ID != c.channelID {
		return nil, platformError("get_user", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapChannel(c.accountID, response.Items[0]), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "video ID is required")
	}
	if err := c.requireScope("get_post", "https://www.googleapis.com/auth/youtube.readonly"); err != nil {
		return nil, err
	}
	query := url.Values{"part": {"snippet,contentDetails,status,statistics"}, "id": {postID}}
	var response videoList
	if err := c.transport.JSON(ctx, http.MethodGet, "/videos", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Items) != 1 || response.Items[0].ID != postID {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapVideo(c.accountID, response.Items[0], c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.channelID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured YouTube channel ID")
	}
	if err := c.requireScope("list_posts", "https://www.googleapis.com/auth/youtube.readonly"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"part": {"snippet"}, "channelId": {c.channelID}, "type": {"video"}, "order": {"date"}}
	if input.Cursor != "" {
		query.Set("pageToken", input.Cursor)
	}
	if input.MaxResults > 0 {
		limit := input.MaxResults
		if limit > 50 {
			limit = 50
		}
		query.Set("maxResults", strconv.Itoa(limit))
	}
	if input.StartTime != nil {
		query.Set("publishedAfter", input.StartTime.UTC().Format(timeRFC3339))
	}
	if input.EndTime != nil {
		query.Set("publishedBefore", input.EndTime.UTC().Format(timeRFC3339))
	}
	var response searchList
	if err := c.transport.JSON(ctx, http.MethodGet, "/search", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Items))
	for _, result := range response.Items {
		if result.ID.VideoID != "" {
			items = append(items, mapSearchResult(c.accountID, result))
		}
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: stringPointer(response.NextPageToken), PrevCursor: stringPointer(response.PrevPageToken), HasMore: response.NextPageToken != ""}, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if input.PostID == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "video ID is required")
	}
	if err := c.requireScope("list_comments", "https://www.googleapis.com/auth/youtube.readonly"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query := url.Values{"part": {"snippet,replies"}, "videoId": {input.PostID}, "textFormat": {"plainText"}, "order": {"time"}}
	if input.Cursor != "" {
		query.Set("pageToken", input.Cursor)
	}
	if input.MaxResults > 0 {
		limit := input.MaxResults
		if limit > 100 {
			limit = 100
		}
		query.Set("maxResults", strconv.Itoa(limit))
	}
	var response commentList
	if err := c.transport.JSON(ctx, http.MethodGet, "/commentThreads", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	observedAt := c.clock.Now()
	var items []socialhub.Comment
	for _, thread := range response.Items {
		items = append(items, mapComment(c.accountID, input.PostID, thread.Snippet.TopLevelComment, observedAt))
		for _, reply := range thread.Replies.Comments {
			items = append(items, mapComment(c.accountID, input.PostID, reply, observedAt))
		}
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: stringPointer(response.NextPageToken), PrevCursor: stringPointer(response.PrevPageToken), HasMore: response.NextPageToken != ""}, nil
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
