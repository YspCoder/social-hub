package twitch

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	target := strings.TrimSpace(userID)
	if target == "" || target == "me" {
		target = c.userID
	}
	if target == "" {
		return nil, invalidArgument("get_user", "user ID is required when account.settings.user_id is not configured")
	}
	var response userPage
	if err := c.get(ctx, "/users", url.Values{"id": {target}}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, platformError("get_user", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapUser(c.accountID, response.Data[0])
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if strings.TrimSpace(postID) == "" {
		return nil, invalidArgument("get_post", "video ID is required")
	}
	var response videoPage
	if err := c.get(ctx, "/videos", url.Values{"id": {postID}}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapVideo(c.accountID, response.Data[0], c.clock.Now())
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" || userID == "me" {
		userID = c.userID
	}
	if userID == "" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID is required when account.settings.user_id is not configured")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Twitch VOD listing exposes coarse period filters, not exact time ranges")
	}
	query := url.Values{"user_id": {userID}}
	if err := setPaging(query, input.Cursor, input.MaxResults); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response videoPage
	if err := c.get(ctx, "/videos", query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return mapVideoPage(c.accountID, response, c.clock.Now())
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Helix does not expose VOD comments or arbitrary chat history")
}
