package dribbble

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if userID != "me" && userID != client.userID {
		return nil, invalidArgument("get_user", "Dribbble API v2 can only fetch the configured OAuth user")
	}
	if err := client.requireScopes("get_user", "public"); err != nil {
		return nil, err
	}
	var response User
	if _, err := client.requestJSON(ctx, http.MethodGet, "/user", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || response.ID != mustID(client.userID) {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(client.accountID, response), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(postID) {
		return nil, invalidArgument("get_post", "Shot ID must be a positive integer")
	}
	if err := client.requireScopes("get_post", "public"); err != nil {
		return nil, err
	}
	var response Shot
	if _, err := client.requestJSON(ctx, http.MethodGet, "/shots/"+postID, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || response.ID != mustID(postID) {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapShot(client.accountID, response, client.clock.Now()), nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	userID := input.UserID
	if userID == "" {
		userID = client.userID
	}
	if userID != client.userID || input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "API v2 lists only the configured user's Shots and does not accept date filters")
	}
	if err := client.requireScopes("list_posts", "public"); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	var response []Shot
	metadata, err := client.requestJSON(ctx, http.MethodGet, "/user/shots", query, nil, &response, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response))
	observedAt := client.clock.Now()
	for _, shot := range response {
		if shot.ID > 0 {
			items = append(items, *mapShot(client.accountID, shot, observedAt))
		}
	}
	next, previous := client.pageCursors(metadata.Header, client.baseURL.Path+"/user/shots")
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (client *Client) ListComments(context.Context, socialhub.ListCommentsRequest, ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	return socialhub.Page[socialhub.Comment]{}, unsupported("list_comments", "Dribbble API v2 removed comment endpoints")
}

func mustID(value string) int64 {
	var id int64
	for _, character := range value {
		id = id*10 + int64(character-'0')
	}
	return id
}
