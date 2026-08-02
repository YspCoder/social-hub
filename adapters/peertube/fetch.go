package peertube

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" {
		userID = c.accountName
	}
	if !validActorHandle(userID) {
		return nil, invalidArgument("get_user", "a valid account name or handle is required")
	}
	var response Account
	if err := c.transport.JSON(ctx, http.MethodGet, "/accounts/"+url.PathEscape(userID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return c.mapAccount(response)
}

func (c *Client) GetVideo(ctx context.Context, videoID string, options ...socialhub.CallOption) (*Video, error) {
	if !validResourceID(videoID) {
		return nil, invalidArgument("get_video", "a valid video ID, UUID, or short UUID is required")
	}
	var response Video
	if err := c.transport.JSON(ctx, http.MethodGet, "/videos/"+url.PathEscape(videoID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID < 1 || response.UUID == "" {
		return nil, platformError("get_video", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	video, err := c.GetVideo(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return c.mapVideo(*video)
}

func (c *Client) ListVideos(ctx context.Context, input VideoListRequest, options ...socialhub.CallOption) (socialhub.Page[Video], error) {
	if input.AccountName != "" && input.ChannelHandle != "" {
		return socialhub.Page[Video]{}, invalidArgument("list_videos", "account name and channel handle are mutually exclusive")
	}
	if input.AccountName != "" && !validActorHandle(input.AccountName) {
		return socialhub.Page[Video]{}, invalidArgument("list_videos", "account name is invalid")
	}
	if input.ChannelHandle != "" && !validActorHandle(input.ChannelHandle) {
		return socialhub.Page[Video]{}, invalidArgument("list_videos", "channel handle is invalid")
	}
	if err := validateSort(input.Sort, "name", "-duration", "-createdAt", "-publishedAt", "-views", "-likes", "-comments", "-trending", "-hot", "-best"); err != nil {
		return socialhub.Page[Video]{}, invalidArgument("list_videos", err.Error())
	}
	query, start, limit, err := pageQuery("list_videos", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[Video]{}, err
	}
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	if strings.TrimSpace(input.Search) != "" {
		query.Set("search", input.Search)
	}
	path := "/videos"
	if input.AccountName != "" {
		path = "/accounts/" + url.PathEscape(input.AccountName) + "/videos"
	} else if input.ChannelHandle != "" {
		path = "/video-channels/" + url.PathEscape(input.ChannelHandle) + "/videos"
	}
	var response VideoListResponse
	if err := c.transport.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[Video]{}, err
	}
	next, previous, hasMore, err := pageCursors(response.Total, start, limit, len(response.Data))
	if err != nil {
		return socialhub.Page[Video]{}, err
	}
	return socialhub.Page[Video]{Items: response.Data, NextCursor: next, PrevCursor: previous, HasMore: hasMore}, nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "PeerTube video listing does not accept exact publication time ranges")
	}
	accountName := input.UserID
	if accountName == "" {
		accountName = c.accountName
	}
	page, err := c.ListVideos(ctx, VideoListRequest{AccountName: accountName, Cursor: input.Cursor, MaxResults: input.MaxResults}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(page.Items))
	for _, video := range page.Items {
		post, err := c.mapVideo(video)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: page.NextCursor, PrevCursor: page.PrevCursor, HasMore: page.HasMore}, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validResourceID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "a valid video ID is required")
	}
	query, start, limit, err := pageQuery("list_comments", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	var response commentThreadResponse
	path := "/videos/" + url.PathEscape(input.PostID) + "/comment-threads"
	if err := c.transport.JSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Data))
	for _, raw := range response.Data {
		comment, err := c.mapComment(input.PostID, raw)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		items = append(items, comment)
	}
	next, previous, hasMore, err := pageCursors(response.Total, start, limit, len(items))
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: hasMore}, nil
}
