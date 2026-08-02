package deviantart

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) WhoAmI(ctx context.Context, options ...socialhub.CallOption) (*User, error) {
	if err := client.requireScopes("who_am_i", "basic", "user"); err != nil {
		return nil, err
	}
	var response User
	if err := client.request(ctx, http.MethodGet, "/user/whoami", nil, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.UserID) || !strings.EqualFold(response.Username, client.username) || client.userID != "" && response.UserID != client.userID {
		return nil, platformError("who_am_i", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) Profile(ctx context.Context, username string, options ...socialhub.CallOption) (*Profile, error) {
	if !validUsername(username) {
		return nil, invalidArgument("profile", "username is invalid")
	}
	if err := client.requireScopes("profile", "browse"); err != nil {
		return nil, err
	}
	var response Profile
	if err := client.request(ctx, http.MethodGet, apiPath("user", "profile", url.PathEscape(username)), nil, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.User.UserID) || !strings.EqualFold(response.User.Username, username) {
		return nil, platformError("profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) GetDeviation(ctx context.Context, deviationID string, options ...socialhub.CallOption) (*Deviation, error) {
	if !validResourceID(deviationID) {
		return nil, invalidArgument("get_deviation", "Deviation ID is invalid")
	}
	if err := client.requireScopes("get_deviation", "browse"); err != nil {
		return nil, err
	}
	var response Deviation
	if err := client.request(ctx, http.MethodGet, apiPath("deviation", url.PathEscape(deviationID)), nil, &response, options...); err != nil {
		return nil, err
	}
	if response.DeviationID != deviationID {
		return nil, platformError("get_deviation", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) ListGallery(ctx context.Context, input GalleryPageRequest, options ...socialhub.CallOption) (*DeviationPage, error) {
	if !validUsername(input.Username) || input.Offset < 0 || input.Offset > 50000 || input.MaxResults < 0 || input.MaxResults > 24 {
		return nil, invalidArgument("list_gallery", "username, offset, or max_results is invalid")
	}
	if err := client.requireScopes("list_gallery", "browse"); err != nil {
		return nil, err
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 10
	}
	query := url.Values{
		"username": {input.Username}, "offset": {strconv.Itoa(input.Offset)}, "limit": {strconv.Itoa(limit)},
	}
	var response DeviationPage
	if err := client.request(ctx, http.MethodGet, "/gallery/all", query, &response, options...); err != nil {
		return nil, err
	}
	if _, _, err := pageCursors(response.NextOffset, response.HasMore, input.Offset, limit, 0, 50000); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) ListProfilePosts(ctx context.Context, input ProfilePostsRequest, options ...socialhub.CallOption) (*ProfilePostPage, error) {
	if !validUsername(input.Username) || input.Cursor != "" && !validCursor(input.Cursor) {
		return nil, invalidArgument("list_profile_posts", "username or cursor is invalid")
	}
	if err := client.requireScopes("list_profile_posts", "browse"); err != nil {
		return nil, err
	}
	query := url.Values{"username": {input.Username}}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var response ProfilePostPage
	if err := client.request(ctx, http.MethodGet, "/user/profile/posts", query, &response, options...); err != nil {
		return nil, err
	}
	if response.HasMore && (response.NextCursor == nil || !validCursor(*response.NextCursor)) || response.PrevCursor != nil && !validCursor(*response.PrevCursor) {
		return nil, platformError("list_profile_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) ListDeviationComments(ctx context.Context, input DeviationCommentsRequest, options ...socialhub.CallOption) (*CommentPage, error) {
	if !validResourceID(input.DeviationID) || input.CommentID != "" && !validResourceID(input.CommentID) ||
		input.MaxDepth < 0 || input.MaxDepth > 5 || input.Offset < -10000 || input.Offset > 10000 || input.MaxResults < 0 || input.MaxResults > 50 {
		return nil, invalidArgument("list_deviation_comments", "Deviation ID, comment ID, depth, offset, or max_results is invalid")
	}
	if err := client.requireScopes("list_deviation_comments", "browse"); err != nil {
		return nil, err
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 10
	}
	query := url.Values{
		"maxdepth": {strconv.Itoa(input.MaxDepth)}, "offset": {strconv.Itoa(input.Offset)}, "limit": {strconv.Itoa(limit)},
	}
	if input.CommentID != "" {
		query.Set("commentid", input.CommentID)
	}
	var response CommentPage
	path := apiPath("comments", "deviation", url.PathEscape(input.DeviationID))
	if err := client.request(ctx, http.MethodGet, path, query, &response, options...); err != nil {
		return nil, err
	}
	if _, _, err := pageCursors(response.NextOffset, response.HasMore, input.Offset, limit, -10000, 10000); err != nil {
		return nil, err
	}
	if response.HasLess && response.PrevOffset == nil || response.PrevOffset != nil && (*response.PrevOffset < -10000 || *response.PrevOffset > 10000) {
		return nil, platformError("list_deviation_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID == "me" {
		response, err := client.WhoAmI(ctx, options...)
		if err != nil {
			return nil, err
		}
		return mapUser(client.accountID, *response, nil), nil
	}
	if userID == "" {
		userID = client.username
	}
	response, err := client.Profile(ctx, userID, options...)
	if err != nil {
		return nil, err
	}
	return mapUser(client.accountID, response.User, response), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	response, err := client.GetDeviation(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return client.mapDeviation(*response)
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "DeviantArt Gallery All does not accept time filters")
	}
	username := input.UserID
	if username == "" {
		username = client.username
	}
	query, offset, err := offsetQuery("list_posts", input.Cursor, input.MaxResults, 24, 0, 50000)
	if err != nil || !validUsername(username) {
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "username is invalid")
	}
	limit, _ := strconv.Atoi(query.Get("limit"))
	response, err := client.ListGallery(ctx, GalleryPageRequest{Username: username, Offset: offset, MaxResults: limit}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Results))
	for _, deviation := range response.Results {
		mapped, mapErr := client.mapDeviation(deviation)
		if mapErr != nil {
			return socialhub.Page[socialhub.Post]{}, mapErr
		}
		items = append(items, *mapped)
	}
	next, previous, err := pageCursors(response.NextOffset, response.HasMore, offset, limit, 0, 50000)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: response.HasMore}, nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	query, offset, err := offsetQuery("list_comments", input.Cursor, input.MaxResults, 50, -10000, 10000)
	if err != nil || !validResourceID(input.PostID) {
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "Post ID is invalid")
	}
	limit, _ := strconv.Atoi(query.Get("limit"))
	response, err := client.ListDeviationComments(ctx, DeviationCommentsRequest{
		DeviationID: input.PostID, Offset: offset, MaxResults: limit,
	}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Thread))
	for _, comment := range response.Thread {
		mapped, mapErr := client.mapComment(input.PostID, comment)
		if mapErr != nil {
			return socialhub.Page[socialhub.Comment]{}, mapErr
		}
		items = append(items, *mapped)
	}
	var next, previous *string
	if response.HasMore && response.NextOffset != nil {
		value := strconv.Itoa(*response.NextOffset)
		next = &value
	}
	if response.HasLess && response.PrevOffset != nil {
		value := strconv.Itoa(*response.PrevOffset)
		previous = &value
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: response.HasMore}, nil
}
