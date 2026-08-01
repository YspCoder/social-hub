package vimeo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

// FeedWorkflow exposes Vimeo's home-feed endpoint without weakening the common
// ListPosts ownership semantics.
type FeedWorkflow interface {
	HomeFeed(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error)
}

func (c *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if err := c.requireScopes("get_user", "public"); err != nil {
		return nil, err
	}
	path := "/me"
	if userID != "" {
		if !validResourceID(userID) {
			return nil, invalidArgument("get_user", "user ID is invalid")
		}
		path = "/users/" + escapedID(userID)
	}
	var response vimeoUser
	if err := c.requestJSON(ctx, http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	user, err := c.mapUser(response)
	if err != nil {
		return nil, err
	}
	if userID != "" && user.ID != userID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response user ID mismatch"))
	}
	if userID == "" && c.userID != "" && user.ID != c.userID {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("configured user ID mismatch"))
	}
	return user, nil
}

func (c *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validResourceID(postID) {
		return nil, invalidArgument("get_post", "video ID is required and must be valid")
	}
	if err := c.requireScopes("get_post", "public"); err != nil {
		return nil, err
	}
	var response vimeoVideo
	if err := c.requestJSON(ctx, http.MethodGet, "/videos/"+escapedID(postID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	post, err := c.mapVideo(response)
	if err != nil {
		return nil, err
	}
	if post.ID != postID {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response video ID mismatch"))
	}
	return post, nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if err := c.requireScopes("list_posts", "public"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query, err := makePageQuery("list_posts", input.Cursor, input.MaxResults, input.StartTime != nil || input.EndTime != nil)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	path := "/me/videos"
	if input.UserID != "" {
		if !validResourceID(input.UserID) {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID is invalid")
		}
		path = "/users/" + escapedID(input.UserID) + "/videos"
	}
	return c.listVideos(ctx, "list_posts", path, query, options...)
}

func (c *Client) HomeFeed(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if err := c.requireScopes("home_feed", "public"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query, err := makePageQuery("home_feed", input.Cursor, input.MaxResults, input.StartTime != nil || input.EndTime != nil)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	path := "/me/feed"
	if input.UserID != "" {
		if !validResourceID(input.UserID) {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("home_feed", "user ID is invalid")
		}
		path = "/users/" + escapedID(input.UserID) + "/feed"
	}
	var response vimeoPage[vimeoActivity]
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data))
	for _, activity := range response.Data {
		post, err := c.mapActivity(activity)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	next, previous, err := pageCursors(response.Paging)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, platformError("home_feed", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (c *Client) listVideos(ctx context.Context, operation, path string, query url.Values, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	var response vimeoPage[vimeoVideo]
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data))
	for _, video := range response.Data {
		post, err := c.mapVideo(video)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	next, previous, err := pageCursors(response.Paging)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validResourceID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "video ID is required and must be valid")
	}
	if err := c.requireScopes("list_comments", "public"); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query, err := makePageQuery("list_comments", input.Cursor, input.MaxResults, false)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	var response vimeoPage[vimeoComment]
	path := "/videos/" + escapedID(input.PostID) + "/comments"
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Data))
	for _, comment := range response.Data {
		mapped, err := c.mapComment(input.PostID, nil, comment)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		items = append(items, mapped)
	}
	next, previous, err := pageCursors(response.Paging)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func makePageQuery(operation, cursor string, maximum int, hasTimeFilter bool) (url.Values, error) {
	if hasTimeFilter {
		return nil, unsupported(operation, "Vimeo list endpoints do not expose portable start/end time filters")
	}
	if maximum < 0 {
		return nil, invalidArgument(operation, "max results must not be negative")
	}
	query := url.Values{}
	if cursor != "" {
		page, err := strconv.Atoi(cursor)
		if err != nil || page <= 0 {
			return nil, invalidArgument(operation, "cursor must be a positive decimal page number")
		}
		query.Set("page", strconv.Itoa(page))
	}
	if maximum > 0 {
		if maximum > 100 {
			maximum = 100
		}
		query.Set("per_page", strconv.Itoa(maximum))
	}
	return query, nil
}

var _ FeedWorkflow = (*Client)(nil)
