package tiktok

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const videoFields = "id,create_time,cover_image_url,share_url,video_description,duration,height,width,title,embed_html,embed_link,like_count,comment_count,share_count,view_count"

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "" && userID != c.openID {
		return nil, invalidArgument("get_user", "user ID must be the configured TikTok open_id")
	}
	if err := c.requireScope("get_user", "user.info.basic"); err != nil {
		return nil, err
	}
	fields := []string{"open_id", "union_id", "avatar_url", "display_name", "profile_deep_link"}
	if contains(c.scopes, "user.info.profile") {
		fields = append(fields, "bio_description", "username", "is_verified")
	}
	if contains(c.scopes, "user.info.stats") {
		fields = append(fields, "follower_count", "following_count", "likes_count", "video_count")
	}
	var response userEnvelope
	if err := c.transport.JSON(ctx, http.MethodGet, "/v2/user/info/", url.Values{"fields": {strings.Join(fields, ",")}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkAPIError("get_user", response.Error); err != nil {
		return nil, err
	}
	if response.Data.User.OpenID == "" || response.Data.User.OpenID != c.openID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(c.accountID, response.Data.User), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if strings.TrimSpace(postID) == "" {
		return nil, invalidArgument("get_post", "video ID is required")
	}
	if err := c.requireScope("get_post", "video.list"); err != nil {
		return nil, err
	}
	body := struct {
		Filters struct {
			VideoIDs []string `json:"video_ids"`
		} `json:"filters"`
	}{}
	body.Filters.VideoIDs = []string{postID}
	var response videoEnvelope
	if err := c.transport.JSON(ctx, http.MethodPost, "/v2/video/query/", url.Values{"fields": {videoFields}}, body, &response, options...); err != nil {
		return nil, err
	}
	if err := checkAPIError("get_post", response.Error); err != nil {
		return nil, err
	}
	if len(response.Data.Videos) != 1 || response.Data.Videos[0].ID != postID {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapVideo(c.accountID, c.openID, response.Data.Videos[0], c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.openID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured TikTok open_id")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "TikTok video.list uses one descending timestamp cursor rather than time ranges")
	}
	if err := c.requireScope("list_posts", "video.list"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	body := struct {
		Cursor   int64 `json:"cursor,omitempty"`
		MaxCount int   `json:"max_count,omitempty"`
	}{}
	if input.Cursor != "" {
		cursor, err := strconv.ParseInt(input.Cursor, 10, 64)
		if err != nil || cursor < 0 {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "cursor must be a non-negative millisecond timestamp")
		}
		body.Cursor = cursor
	}
	if input.MaxResults > 0 {
		body.MaxCount = input.MaxResults
		if body.MaxCount > 20 {
			body.MaxCount = 20
		}
	}
	var response videoEnvelope
	if err := c.transport.JSON(ctx, http.MethodPost, "/v2/video/list/", url.Values{"fields": {videoFields}}, body, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if err := checkAPIError("list_posts", response.Error); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data.Videos))
	observedAt := c.clock.Now()
	for _, video := range response.Data.Videos {
		items = append(items, *mapVideo(c.accountID, c.openID, video, observedAt))
	}
	var next *string
	if response.Data.HasMore {
		value := strconv.FormatInt(response.Data.Cursor, 10)
		next = &value
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: response.Data.HasMore}, nil
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "comment reads are part of separately restricted Research APIs, not Display API")
}
