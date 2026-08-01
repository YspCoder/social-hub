package stackexchange

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	if !validID(userID) {
		return nil, invalidArgument("get_user", "user ID must be a positive integer")
	}
	response, err := call[UserDetails](client, ctx, "users", http.MethodGet, "/users/"+userID, nil, nil, options...)
	if err != nil {
		return nil, err
	}
	if len(response.Items) == 0 {
		return nil, platformError("get_user", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if response.Items[0].UserID <= 0 {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapUser(client.accountID, response.Items[0]), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(postID) {
		return nil, invalidArgument("get_post", "post ID must be a positive integer")
	}
	query := url.Values{"filter": {"withbody"}}
	response, err := call[PostDetails](client, ctx, "posts", http.MethodGet, "/posts/"+postID, query, nil, options...)
	if err != nil {
		return nil, err
	}
	if len(response.Items) == 0 {
		return nil, platformError("get_post", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if firstPositive(response.Items[0].PostID, response.Items[0].QuestionID, response.Items[0].AnswerID) <= 0 {
		return nil, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response.Items[0], client.clock.Now()), nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		userID = client.userID
	}
	if !validID(userID) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be supplied or configured as a positive integer")
	}
	if input.StartTime != nil && input.EndTime != nil && input.StartTime.After(*input.EndTime) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "start_time must not be after end_time")
	}
	query, page, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query.Set("filter", "withbody")
	query.Set("order", "desc")
	query.Set("sort", "activity")
	if input.StartTime != nil {
		query.Set("fromdate", strconv.FormatInt(input.StartTime.Unix(), 10))
	}
	if input.EndTime != nil {
		query.Set("todate", strconv.FormatInt(input.EndTime.Unix(), 10))
	}
	response, err := call[PostDetails](client, ctx, "user_questions", http.MethodGet, "/users/"+userID+"/questions", query, nil, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Items))
	observedAt := client.clock.Now()
	for _, item := range response.Items {
		if firstPositive(item.PostID, item.QuestionID) <= 0 {
			continue
		}
		items = append(items, *mapPost(client.accountID, item, observedAt))
	}
	return pageFrom(items, page, response.HasMore), nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be a positive integer")
	}
	query, page, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query.Set("filter", "withbody")
	query.Set("order", "asc")
	query.Set("sort", "creation")
	response, err := call[CommentDetails](client, ctx, "post_comments", http.MethodGet, "/posts/"+input.PostID+"/comments", query, nil, options...)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items := make([]socialhub.Comment, 0, len(response.Items))
	observedAt := client.clock.Now()
	for _, item := range response.Items {
		if item.CommentID <= 0 || item.PostID <= 0 {
			continue
		}
		items = append(items, *mapComment(client.accountID, item, observedAt))
	}
	return pageFrom(items, page, response.HasMore), nil
}
