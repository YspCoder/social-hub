package discourse

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	api, err := client.requireAPI("get_user")
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(userID)
	if username == "" {
		username = client.apiUsername
	}
	if !validUsername(username) {
		return nil, invalidArgument("get_user", "Discourse username is invalid")
	}
	var response userResponse
	if err := api.JSON(ctx, http.MethodGet, path("u", username), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.User.ID <= 0 || response.User.Username == "" {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return client.mapUser(response.User), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	post, err := client.getPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return client.mapPost(post), nil
}

func (client *Client) getPost(ctx context.Context, postID string, options ...socialhub.CallOption) (discoursePost, error) {
	api, err := client.requireAPI("get_post")
	if err != nil {
		return discoursePost{}, err
	}
	if !validID(postID) {
		return discoursePost{}, invalidArgument("get_post", "post ID must be a positive integer")
	}
	var response discoursePost
	if err := api.JSON(ctx, http.MethodGet, path("posts", postID), nil, nil, &response, options...); err != nil {
		return discoursePost{}, err
	}
	if response.ID != mustID(postID) {
		return discoursePost{}, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response, nil
}

func (client *Client) ListPosts(context.Context, socialhub.ListPostsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	return socialhub.Page[socialhub.Post]{}, unsupported(
		"list_posts",
		"Discourse /posts.json is a site-wide feed, not an account feed; use TopicWorkflow.ListLatestPosts",
	)
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	api, err := client.requireAPI("list_comments")
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if !validID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be a positive integer")
	}
	if input.Cursor != "" {
		return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Discourse post replies do not expose cursor pagination")
	}
	if input.MaxResults < 0 {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "max_results must not be negative")
	}
	var response []discoursePost
	if err := api.JSON(ctx, http.MethodGet, path("posts", input.PostID, "replies"), nil, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	limit := len(response)
	if input.MaxResults > 0 && input.MaxResults < limit {
		limit = input.MaxResults
	}
	items := make([]socialhub.Comment, 0, limit)
	for _, post := range response[:limit] {
		if post.ID <= 0 {
			return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		items = append(items, client.mapComment(input.PostID, post))
	}
	return socialhub.Page[socialhub.Comment]{Items: items}, nil
}

func (client *Client) ListLatestPosts(ctx context.Context, before string, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	api, err := client.requireAPI("list_latest_posts")
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if !validCursor(before) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_latest_posts", "before cursor must be a positive post ID")
	}
	query := url.Values{}
	if before != "" {
		query.Set("before", before)
	}
	var response latestPostsResponse
	if err := api.JSON(ctx, http.MethodGet, "/posts.json", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	page := socialhub.Page[socialhub.Post]{Items: make([]socialhub.Post, 0, len(response.Posts))}
	var minimum int64
	for _, post := range response.Posts {
		if post.ID <= 0 {
			return socialhub.Page[socialhub.Post]{}, platformError("list_latest_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		page.Items = append(page.Items, *client.mapPost(post))
		if minimum == 0 || post.ID < minimum {
			minimum = post.ID
		}
	}
	if minimum > 0 {
		next := strconv.FormatInt(minimum, 10)
		page.NextCursor, page.HasMore = &next, true
	}
	return page, nil
}
