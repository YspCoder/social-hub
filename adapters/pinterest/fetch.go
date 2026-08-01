package pinterest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" {
		userID = c.userID
	}
	if userID != c.userID {
		return nil, invalidArgument("get_user", "user ID must be the configured Pinterest account")
	}
	if err := c.requireScopes("get_user", "user_accounts:read"); err != nil {
		return nil, err
	}
	var response pinterestAccount
	if err := c.transport.JSON(ctx, http.MethodGet, "/user_account", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || response.ID != c.userID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapAccount(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if postID == "" {
		return nil, invalidArgument("get_post", "Pin ID is required")
	}
	if err := c.requireScopes("get_post", "pins:read"); err != nil {
		return nil, err
	}
	var response pinterestPin
	if err := c.transport.JSON(ctx, http.MethodGet, "/pins/"+url.PathEscape(postID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != postID {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPin(c.accountID, c.userID, response, c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.userID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured Pinterest account")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Pinterest List Pins does not accept time-range filters")
	}
	if err := c.requireScopes("list_posts", "boards:read", "pins:read"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{}
	if input.Cursor != "" {
		query.Set("bookmark", input.Cursor)
	}
	if input.MaxResults > 0 {
		limit := input.MaxResults
		if limit > 250 {
			limit = 250
		}
		query.Set("page_size", strconv.Itoa(limit))
	}
	var response pinList
	if err := c.transport.JSON(ctx, http.MethodGet, "/pins", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Items))
	for _, pin := range response.Items {
		items = append(items, *mapPin(c.accountID, c.userID, pin, c.clock.Now()))
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: response.Bookmark, HasMore: response.Bookmark != nil && *response.Bookmark != ""}, nil
}

func (c *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Pinterest API v5 does not expose organic Pin comment listing")
}
