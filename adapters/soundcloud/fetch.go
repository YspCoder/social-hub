package soundcloud

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, userURN string, options ...socialhub.CallOption) (*socialhub.User, error) {
	path := "/me"
	if userURN != "" {
		if !validURN(userURN, "users") {
			return nil, invalidArgument("get_user", "user ID must be a SoundCloud user URN")
		}
		path = "/users/" + escapedURN(userURN)
	}
	var response soundCloudUser
	if err := c.requestJSON(ctx, http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	user, err := c.mapUser(response)
	if err != nil {
		return nil, err
	}
	if userURN != "" && user.ID != userURN {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response user URN mismatch"))
	}
	if userURN == "" && c.userURN != "" && user.ID != c.userURN {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("configured user URN mismatch"))
	}
	return user, nil
}

func (c *Client) GetPost(ctx context.Context, trackURN string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validURN(trackURN, "tracks") {
		return nil, invalidArgument("get_post", "post ID must be a SoundCloud track URN")
	}
	var response soundCloudTrack
	if err := c.requestJSON(ctx, http.MethodGet, "/tracks/"+escapedURN(trackURN), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	post, err := c.mapTrack(response)
	if err != nil {
		return nil, err
	}
	if post.ID != trackURN {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response track URN mismatch"))
	}
	return post, nil
}

func (c *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "SoundCloud user track endpoints do not expose start/end time filters")
	}
	query, err := pageQuery("list_posts", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	path := "/me/tracks"
	if input.UserID != "" {
		if !validURN(input.UserID, "users") {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be a SoundCloud user URN")
		}
		path = "/users/" + escapedURN(input.UserID) + "/tracks"
	}
	var response soundCloudPage[soundCloudTrack]
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Collection))
	for _, track := range response.Collection {
		post, err := c.mapTrack(track)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	next, err := paginationCursor(response.NextHref, c.apiBaseURL)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, platformError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func (c *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validURN(input.PostID, "tracks") {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be a SoundCloud track URN")
	}
	query, err := pageQuery("list_comments", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	var response soundCloudPage[soundCloudComment]
	path := "/tracks/" + escapedURN(input.PostID) + "/comments"
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Collection))
	for _, comment := range response.Collection {
		mapped, err := c.mapComment(input.PostID, comment)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		items = append(items, mapped)
	}
	next, err := paginationCursor(response.NextHref, c.apiBaseURL)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, HasMore: next != nil}, nil
}

func pageQuery(operation, cursor string, maximum int) (url.Values, error) {
	if maximum < 0 {
		return nil, invalidArgument(operation, "max results must not be negative")
	}
	query := url.Values{"linked_partitioning": {"true"}}
	if cursor != "" {
		if !validCursor(cursor) {
			return nil, invalidArgument(operation, "cursor is invalid")
		}
		query.Set("cursor", cursor)
	}
	if maximum > 0 {
		query.Set("limit", strconv.Itoa(min(maximum, 200)))
	}
	return query, nil
}

func validCursor(cursor string) bool {
	if cursor == "" || len(cursor) > 2048 {
		return false
	}
	return !strings.ContainsFunc(cursor, unicode.IsControl)
}
