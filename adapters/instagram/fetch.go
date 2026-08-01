package instagram

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const mediaFields = "id,caption,media_type,media_product_type,media_url,permalink,thumbnail_url,timestamp,username,children{id,media_type,media_product_type,media_url,permalink,thumbnail_url,timestamp}"

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" {
		userID = c.userID
	}
	if userID != c.userID {
		return nil, invalidArgument("get_user", "user ID must be the configured Instagram professional account")
	}
	if err := c.requireScope("get_user", "instagram_business_basic"); err != nil {
		return nil, err
	}
	query := url.Values{"fields": {"id,user_id,username,name,account_type,profile_picture_url,followers_count,media_count"}}
	var response instagramUser
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(c.userID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" && response.UserID == "" {
		return nil, wrapError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "media ID is required")
	}
	if err := c.requireScope("get_post", "instagram_business_basic"); err != nil {
		return nil, err
	}
	var response instagramMedia
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(postID), url.Values{"fields": {mediaFields}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, wrapError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapMediaPost(c.accountID, c.userID, response), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.userID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured Instagram professional account")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "the Instagram media edge does not accept time-range filters")
	}
	if err := c.requireScope("list_posts", "instagram_business_basic"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"fields": {mediaFields}}
	setPaging(query, input.Cursor, input.MaxResults)
	var response instagramMediaList
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(c.userID)+"/media", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapMediaPage(c.accountID, c.userID, response), nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if input.PostID == "" {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "media ID is required")
	}
	if err := c.requireScope("list_comments", "instagram_business_manage_comments"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query := url.Values{"fields": {"id,text,timestamp,username,from,parent_id,like_count"}}
	setPaging(query, input.Cursor, input.MaxResults)
	var response instagramCommentList
	if err := c.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(input.PostID)+"/comments", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	return mapCommentPage(c.accountID, input.PostID, response, c.clock.Now()), nil
}

func setPaging(query url.Values, cursor string, limit int) {
	if cursor != "" {
		query.Set("after", cursor)
	}
	if limit > 0 {
		if limit > 100 {
			limit = 100
		}
		query.Set("limit", strconv.Itoa(limit))
	}
}
