package wordpresscom

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "" && userID != "me" && (client.userID == "" || userID != client.userID) {
		return nil, invalidArgument("get_user", "WordPress.com REST API can only fetch the authenticated user")
	}
	api, err := client.requireUser("get_user")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("get_user", "users"); err != nil {
		return nil, err
	}
	var response wpUser
	if err := api.JSON(ctx, http.MethodGet, "/me", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || client.userID != "" && response.ID != mustID(client.userID) {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(client.accountID, response), nil
}

func (client *Client) GetSite(ctx context.Context, options ...socialhub.CallOption) (*Site, error) {
	var response Site
	if err := client.readAPI().JSON(ctx, http.MethodGet, client.sitePath(), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || !client.matchesSite(response.ID) {
		return nil, platformError("get_site", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(postID) {
		return nil, invalidArgument("get_post", "Post ID must be a positive integer")
	}
	var response wpPost
	if err := client.readAPI().JSON(ctx, http.MethodGet, client.sitePath("posts", postID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != mustID(postID) || response.SiteID <= 0 || !client.matchesSite(response.SiteID) {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response, client.clock.Now()), nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.UserID != "" && !validID(input.UserID) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "author user ID must be a positive integer")
	}
	if input.StartTime != nil && input.EndTime != nil && input.StartTime.After(*input.EndTime) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "start_time must not be after end_time")
	}
	query, err := pageQuery(input.Cursor, input.MaxResults, input.StartTime, input.EndTime)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	if input.UserID != "" {
		query.Set("author", input.UserID)
	}
	var response postListResponse
	if err := client.readAPI().JSON(ctx, http.MethodGet, client.sitePath("posts"), query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Posts))
	observedAt := client.clock.Now()
	for _, post := range response.Posts {
		if post.ID > 0 && post.SiteID > 0 && client.matchesSite(post.SiteID) {
			items = append(items, *mapPost(client.accountID, post, observedAt))
		}
	}
	page := socialhub.Page[socialhub.Post]{Items: items}
	if response.Meta.NextPage != "" {
		if !validCursor(response.Meta.NextPage) {
			return socialhub.Page[socialhub.Post]{}, platformError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		next := response.Meta.NextPage
		page.NextCursor, page.HasMore = &next, true
	}
	return page, nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validID(input.PostID) || input.MaxResults < 0 {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "Post ID and non-negative max_results are required")
	}
	pageNumber := 1
	if input.Cursor != "" {
		if !validID(input.Cursor) {
			return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "cursor must be a positive page number")
		}
		pageNumber = int(mustID(input.Cursor))
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := url.Values{"number": {strconv.Itoa(limit)}, "page": {strconv.Itoa(pageNumber)}, "type": {"comment"}}
	var response commentListResponse
	if err := client.readAPI().JSON(ctx, http.MethodGet, client.sitePath("posts", input.PostID, "replies"), query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if response.SiteID > 0 && !client.matchesSite(response.SiteID) {
		return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	items := make([]socialhub.Comment, 0, len(response.Comments))
	observedAt := client.clock.Now()
	for _, comment := range response.Comments {
		if comment.ID > 0 {
			items = append(items, mapComment(client.accountID, input.PostID, comment, observedAt))
		}
	}
	page := socialhub.Page[socialhub.Comment]{Items: items}
	if int64(pageNumber*limit) < response.Found && len(items) > 0 {
		next := strconv.Itoa(pageNumber + 1)
		page.NextCursor, page.HasMore = &next, true
	}
	if pageNumber > 1 {
		previous := strconv.Itoa(pageNumber - 1)
		page.PrevCursor = &previous
	}
	return page, nil
}

func (client *Client) matchesSite(id int64) bool {
	if !validID(client.site) {
		return id > 0
	}
	return id == mustID(client.site)
}
