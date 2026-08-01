package reddit

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "" {
		userID = c.userID
	}
	if userID != c.userID {
		return nil, invalidArgument("get_user", "user ID must be the configured Reddit account fullname")
	}
	if err := c.requireScopes("get_user", "identity"); err != nil {
		return nil, err
	}
	var response redditUser
	if err := c.json(ctx, http.MethodGet, "/api/v1/me", url.Values{"raw_json": {"1"}}, &response, options...); err != nil {
		return nil, err
	}
	if fullname(response.ID, "t2_") != c.userID || response.Name != c.username {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(c.accountID, response), nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	postID = fullname(postID, "t3_")
	if !validFullname(postID, "t3_") {
		return nil, invalidArgument("get_post", "Reddit submission fullname is required")
	}
	if err := c.requireScopes("get_post", "read"); err != nil {
		return nil, err
	}
	var response redditListing
	if err := c.json(ctx, http.MethodGet, "/api/info", url.Values{"id": {postID}, "raw_json": {"1"}}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data.Children) != 1 || response.Data.Children[0].Kind != "t3" || fullname(response.Data.Children[0].Data.Name, "t3_") != postID {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	return mapPost(c.accountID, response.Data.Children[0].Data, c.clock.Now()), nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && input.UserID != c.userID {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be the configured Reddit account fullname")
	}
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Reddit user listings do not accept exact time-range filters")
	}
	if err := c.requireScopes("list_posts", "history"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query := url.Values{"raw_json": {"1"}}
	if input.Cursor != "" {
		query.Set("after", input.Cursor)
	}
	if input.MaxResults > 0 {
		limit := input.MaxResults
		if limit > 100 {
			limit = 100
		}
		query.Set("limit", strconv.Itoa(limit))
	}
	var response redditListing
	path := "/user/" + url.PathEscape(c.username) + "/submitted"
	if err := c.json(ctx, http.MethodGet, path, query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data.Children))
	for _, child := range response.Data.Children {
		if child.Kind == "t3" {
			items = append(items, *mapPost(c.accountID, child.Data, c.clock.Now()))
		}
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: response.Data.After, PrevCursor: response.Data.Before, HasMore: response.Data.After != nil && *response.Data.After != ""}, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	postID := fullname(input.PostID, "t3_")
	if !validFullname(postID, "t3_") {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "Reddit submission fullname is required")
	}
	if input.Cursor != "" {
		return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Reddit comment trees do not use listing cursors; morechildren expansion is not included")
	}
	if err := c.requireScopes("list_comments", "read"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query := url.Values{"raw_json": {"1"}, "depth": {"10"}}
	if input.MaxResults > 0 {
		limit := input.MaxResults
		if limit > 500 {
			limit = 500
		}
		query.Set("limit", strconv.Itoa(limit))
	}
	var response []redditListing
	path := "/comments/" + strings.TrimPrefix(postID, "t3_")
	if err := c.json(ctx, http.MethodGet, path, query, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if len(response) < 2 {
		return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	items := mapComments(c.accountID, postID, response[1].Data.Children, c.clock.Now())
	return socialhub.Page[socialhub.Comment]{Items: items}, nil
}
