package mixcloud

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) CurrentUser(ctx context.Context, options ...socialhub.CallOption) (*User, error) {
	var response User
	if err := client.request(ctx, http.MethodGet, "/me/", nil, nil, "", &response, options...); err != nil {
		return nil, err
	}
	username, _, ok := parseUserKey(response.Key)
	if !ok || !strings.EqualFold(username, client.username) {
		return nil, platformError("current_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("configured Mixcloud username mismatch"))
	}
	if _, err := client.mapUser(response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) GetMixcloudUser(ctx context.Context, username string, options ...socialhub.CallOption) (*User, error) {
	username, _, ok := parseUserKey(username)
	if !ok {
		return nil, invalidArgument("get_mixcloud_user", "username or user key is invalid")
	}
	var response User
	if err := client.request(ctx, http.MethodGet, "/"+username+"/", nil, nil, "", &response, options...); err != nil {
		return nil, err
	}
	responseUsername, _, responseOK := parseUserKey(response.Key)
	if !responseOK || !strings.EqualFold(responseUsername, username) {
		return nil, platformError("get_mixcloud_user", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("Mixcloud user response mismatch"))
	}
	if _, err := client.mapUser(response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) GetCloudcast(ctx context.Context, cloudcastKey string, options ...socialhub.CallOption) (*Cloudcast, error) {
	username, slug, key, ok := parseCloudcastKey(cloudcastKey)
	if !ok {
		return nil, invalidArgument("get_cloudcast", "Cloudcast key must contain a username and slug")
	}
	var response Cloudcast
	if err := client.request(ctx, http.MethodGet, "/"+username+"/"+slug+"/", nil, nil, "", &response, options...); err != nil {
		return nil, err
	}
	_, _, responseKey, responseOK := parseCloudcastKey(response.Key)
	if !responseOK || !strings.EqualFold(responseKey, key) {
		return nil, platformError("get_cloudcast", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("Mixcloud Cloudcast response mismatch"))
	}
	if _, err := client.mapCloudcast(response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) ListUserCloudcasts(ctx context.Context, user string, input PageRequest, options ...socialhub.CallOption) (*CloudcastPage, error) {
	username, _, ok := parseUserKey(user)
	if !ok {
		return nil, invalidArgument("list_user_cloudcasts", "username or user key is invalid")
	}
	query, err := pageQuery("list_user_cloudcasts", input, true)
	if err != nil {
		return nil, err
	}
	var response CloudcastPage
	if err := client.request(ctx, http.MethodGet, "/"+username+"/cloudcasts/", query, nil, "", &response, options...); err != nil {
		return nil, err
	}
	for _, item := range response.Data {
		itemUsername, _, _, valid := parseCloudcastKey(item.Key)
		if !valid || !strings.EqualFold(itemUsername, username) {
			return nil, platformError("list_user_cloudcasts", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("Cloudcast owner does not match requested user"))
		}
		if _, err := client.mapCloudcast(item); err != nil {
			return nil, err
		}
	}
	if _, _, err := pageCursors(response.Paging, client.apiBaseURL); err != nil {
		return nil, platformError("list_user_cloudcasts", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	response.Paging, err = sanitizedPaging(response.Paging, client.apiBaseURL)
	if err != nil {
		return nil, platformError("list_user_cloudcasts", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &response, nil
}

func (client *Client) ListCloudcastComments(ctx context.Context, cloudcastKey string, input PageRequest, options ...socialhub.CallOption) (*CommentPage, error) {
	username, slug, key, ok := parseCloudcastKey(cloudcastKey)
	if !ok {
		return nil, invalidArgument("list_cloudcast_comments", "Cloudcast key must contain a username and slug")
	}
	query, err := pageQuery("list_cloudcast_comments", input, false)
	if err != nil {
		return nil, err
	}
	var response CommentPage
	path := "/" + username + "/" + slug + "/comments/"
	if err := client.request(ctx, http.MethodGet, path, query, nil, "", &response, options...); err != nil {
		return nil, err
	}
	for _, item := range response.Data {
		if _, err := client.mapComment(key, item); err != nil {
			return nil, err
		}
	}
	if _, _, err := pageCursors(response.Paging, client.apiBaseURL); err != nil {
		return nil, platformError("list_cloudcast_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	response.Paging, err = sanitizedPaging(response.Paging, client.apiBaseURL)
	if err != nil {
		return nil, platformError("list_cloudcast_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &response, nil
}

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	var response *User
	var err error
	if userID == "" || userID == "me" {
		response, err = client.CurrentUser(ctx, options...)
	} else {
		response, err = client.GetMixcloudUser(ctx, userID, options...)
	}
	if err != nil {
		return nil, err
	}
	return client.mapUser(*response)
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	response, err := client.GetCloudcast(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return client.mapCloudcast(*response)
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	user := input.UserID
	if user == "" {
		user = client.username
	}
	response, err := client.ListUserCloudcasts(ctx, user, PageRequest{
		Cursor: input.Cursor, MaxResults: input.MaxResults, StartTime: input.StartTime, EndTime: input.EndTime,
	}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Data))
	for _, cloudcast := range response.Data {
		post, err := client.mapCloudcast(cloudcast)
		if err != nil {
			return socialhub.Page[socialhub.Post]{}, err
		}
		items = append(items, *post)
	}
	next, previous, err := pageCursors(response.Paging, client.apiBaseURL)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, platformError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	_, _, key, ok := parseCloudcastKey(input.PostID)
	if !ok {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be a Mixcloud Cloudcast key")
	}
	response, err := client.ListCloudcastComments(ctx, key, PageRequest{Cursor: input.Cursor, MaxResults: input.MaxResults}, options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Data))
	for _, comment := range response.Data {
		mapped, err := client.mapComment(key, comment)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		items = append(items, mapped)
	}
	next, previous, err := pageCursors(response.Paging, client.apiBaseURL)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func pageQuery(operation string, input PageRequest, allowTime bool) (url.Values, error) {
	offset, ok := parseOffset(input.Cursor)
	if !ok || input.MaxResults < 0 || input.MaxResults > 100 {
		return nil, invalidArgument(operation, "cursor or max_results is invalid")
	}
	if !allowTime && (input.StartTime != nil || input.EndTime != nil) {
		return nil, unsupported(operation, "this Mixcloud connection does not accept time filters")
	}
	if input.StartTime != nil && input.StartTime.Unix() < 0 || input.EndTime != nil && input.EndTime.Unix() < 0 ||
		input.StartTime != nil && input.EndTime != nil && input.StartTime.After(*input.EndTime) {
		return nil, invalidArgument(operation, "time filters are invalid")
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 20
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}
	if input.StartTime != nil {
		query.Set("since", strconv.FormatInt(input.StartTime.Unix(), 10))
	}
	if input.EndTime != nil {
		query.Set("until", strconv.FormatInt(input.EndTime.Unix(), 10))
	}
	return query, nil
}
