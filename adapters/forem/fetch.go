package forem

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	path := "/api/users/me"
	if strings.TrimSpace(userID) != "" {
		if !validIdentifier(userID) {
			return nil, invalidArgument("get_user", "user ID or username is invalid")
		}
		path = resourcePath("users", userID)
	}
	var response wireUser
	if err := client.requestJSON(ctx, http.MethodGet, path, nil, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.identifier() <= 0 || response.Username == "" {
		return nil, platformError("get_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return client.mapUser(response), nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	article, err := client.getArticle(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return client.mapPost(article), nil
}

func (client *Client) getArticle(ctx context.Context, articleID string, options ...socialhub.CallOption) (wireArticle, error) {
	if !validID(articleID) {
		return wireArticle{}, invalidArgument("get_article", "article ID must be a positive integer")
	}
	var response wireArticle
	if err := client.requestJSON(ctx, http.MethodGet, resourcePath("articles", articleID), nil, nil, &response, options...); err != nil {
		return wireArticle{}, err
	}
	if response.ID != mustID(articleID) {
		return wireArticle{}, platformError("get_article", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response, nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Forem Article pagination does not accept exact time ranges")
	}
	query, pageNumber, pageSize, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	path := "/api/articles/me"
	if strings.TrimSpace(input.UserID) != "" {
		if !validIdentifier(input.UserID) {
			return socialhub.Page[socialhub.Post]{}, invalidArgument("list_posts", "user ID must be a Forem username")
		}
		path = "/api/articles"
		query.Set("username", input.UserID)
	}
	var response []wireArticle
	if err := client.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	result := socialhub.Page[socialhub.Post]{Items: make([]socialhub.Post, 0, len(response))}
	for _, article := range response {
		if article.ID <= 0 {
			return socialhub.Page[socialhub.Post]{}, platformError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.Items = append(result.Items, *client.mapPost(article))
	}
	result.NextCursor, result.PrevCursor, result.HasMore = pageCursors(len(response), pageNumber, pageSize)
	return result, nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "article ID must be a positive integer")
	}
	query, pageNumber, pageSize, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query.Set("a_id", input.PostID)
	var response []wireComment
	if err := client.requestJSON(ctx, http.MethodGet, "/api/comments", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	result := socialhub.Page[socialhub.Comment]{Items: make([]socialhub.Comment, 0, len(response))}
	if err := client.appendComments(&result.Items, input.PostID, "", response); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	result.NextCursor, result.PrevCursor, result.HasMore = pageCursors(len(response), pageNumber, pageSize)
	return result, nil
}

func validCommentID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 || strings.ContainsAny(value, "/\\?#\x00\r\n") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}
