package lemmy

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*socialhub.User, error) {
	identifier := strings.TrimSpace(userID)
	if identifier == "" {
		identifier = client.username
	}
	response, err := client.getPersonDetails(ctx, identifier, nil, options...)
	if err != nil {
		return nil, err
	}
	return client.mapUser(response.PersonView), nil
}

func (client *Client) getPersonDetails(ctx context.Context, identifier string, page url.Values, options ...socialhub.CallOption) (personDetailsResponse, error) {
	query := url.Values{}
	for key, values := range page {
		query[key] = append([]string(nil), values...)
	}
	if validID(identifier) {
		query.Set("person_id", identifier)
	} else {
		if !validUsername(identifier) {
			return personDetailsResponse{}, invalidArgument("get_person", "person ID or username is invalid")
		}
		query.Set("username", identifier)
	}
	var response personDetailsResponse
	if err := client.requestJSON(ctx, http.MethodGet, "/person", query, nil, &response, options...); err != nil {
		return personDetailsResponse{}, err
	}
	if response.PersonView.Person.ID <= 0 || response.PersonView.Person.Name == "" {
		return personDetailsResponse{}, platformError("get_person", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if validID(identifier) && response.PersonView.Person.ID != mustID(identifier) {
		return personDetailsResponse{}, platformError("get_person", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response, nil
}

func (client *Client) GetPost(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	view, err := client.getPostView(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	mapped := client.mapPost(view)
	return &mapped.Common, nil
}

func (client *Client) getPostView(ctx context.Context, postID string, options ...socialhub.CallOption) (wirePostView, error) {
	if !validID(postID) {
		return wirePostView{}, invalidArgument("get_post", "post ID must be a positive integer")
	}
	query := url.Values{"id": {postID}}
	var response getPostResponse
	if err := client.requestJSON(ctx, http.MethodGet, "/post", query, nil, &response, options...); err != nil {
		return wirePostView{}, err
	}
	if !validPostView(response.PostView) || response.PostView.Post.ID != mustID(postID) {
		return wirePostView{}, platformError("get_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return response.PostView, nil
}

func (client *Client) ListPosts(ctx context.Context, input socialhub.ListPostsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	if input.StartTime != nil || input.EndTime != nil {
		return socialhub.Page[socialhub.Post]{}, unsupported("list_posts", "Lemmy API v3 person pages do not accept exact time ranges")
	}
	query, pageNumber, pageSize, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	identifier := strings.TrimSpace(input.UserID)
	if identifier == "" {
		identifier = client.username
	}
	response, err := client.getPersonDetails(ctx, identifier, query, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	result := socialhub.Page[socialhub.Post]{Items: make([]socialhub.Post, 0, len(response.Posts))}
	for _, view := range response.Posts {
		if !validPostView(view) {
			return socialhub.Page[socialhub.Post]{}, platformError("list_posts", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		result.Items = append(result.Items, client.mapPost(view).Common)
	}
	result.NextCursor, result.PrevCursor, result.HasMore = pageCursors(len(response.Posts), pageNumber, pageSize)
	return result, nil
}

func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validID(input.PostID) {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "post ID must be a positive integer")
	}
	query, pageNumber, pageSize, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	query.Set("post_id", input.PostID)
	query.Set("sort", "Old")
	var response getCommentsResponse
	if err := client.requestJSON(ctx, http.MethodGet, "/comment/list", query, nil, &response, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	result := socialhub.Page[socialhub.Comment]{Items: make([]socialhub.Comment, 0, len(response.Comments))}
	for _, view := range response.Comments {
		if view.Comment.PostID != mustID(input.PostID) {
			return socialhub.Page[socialhub.Comment]{}, platformError("list_comments", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		comment, err := client.mapComment(view)
		if err != nil {
			return socialhub.Page[socialhub.Comment]{}, err
		}
		result.Items = append(result.Items, comment)
	}
	result.NextCursor, result.PrevCursor, result.HasMore = pageCursors(len(response.Comments), pageNumber, pageSize)
	return result, nil
}

func validPostView(view wirePostView) bool {
	return view.Post.ID > 0 && view.Post.CreatorID > 0 && view.Post.CommunityID > 0 &&
		view.Creator.ID > 0 && view.Community.ID > 0 && view.Post.Name != ""
}
