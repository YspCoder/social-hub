package kitsu

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListComments(ctx context.Context, input CommentsRequest, options ...socialhub.CallOption) (socialhub.Page[Comment], error) {
	if !validID(input.PostID) {
		return socialhub.Page[Comment]{}, invalidArgument("list_comments", "post ID is invalid")
	}
	offset, query, err := pagination(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[Comment]{}, err
	}
	query.Set("filter[postId]", input.PostID)
	query.Set("include", "user")
	query.Set("sort", "createdAt")
	var document collectionDocument
	if err := c.request(ctx, "list_comments", http.MethodGet, "comments", query, nil, &document, options...); err != nil {
		return socialhub.Page[Comment]{}, err
	}
	index := includedIndex(document.Included)
	items := make([]Comment, 0, len(document.Data))
	for _, item := range document.Data {
		decoded, err := decodeComment(item, index)
		if err != nil {
			return socialhub.Page[Comment]{}, err
		}
		items = append(items, decoded)
	}
	limit := input.Limit
	if limit == 0 {
		limit = maxPageSize
	}
	return toPage(items, document.Links, offset, limit)
}

func (c *Client) GetComment(ctx context.Context, id string, options ...socialhub.CallOption) (*Comment, error) {
	if !validID(id) {
		return nil, invalidArgument("get_comment", "comment ID is invalid")
	}
	var document resourceDocument
	query := url.Values{"include": {"user"}}
	if err := c.request(ctx, "get_comment", http.MethodGet, "comments/"+url.PathEscape(id), query, nil, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodeComment(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateComment(ctx context.Context, input CreateCommentRequest, options ...socialhub.CallOption) (*Comment, error) {
	if err := c.requireUser("create_comment"); err != nil {
		return nil, err
	}
	if !validID(input.PostID) || input.ParentID != "" && !validID(input.ParentID) || !validText(input.Content) {
		return nil, invalidArgument("create_comment", "post ID, parent ID, or content is invalid")
	}
	relations := map[string]relationship{
		"post": identifierRelationship("posts", input.PostID),
		"user": identifierRelationship("users", c.userID),
	}
	if input.ParentID != "" {
		relations["parent"] = identifierRelationship("comments", input.ParentID)
	}
	request := mutationDocument{Data: mutationResource{
		Type: "comments", Attributes: map[string]any{"content": input.Content}, Relationships: relations,
	}}
	var document resourceDocument
	if err := c.request(ctx, "create_comment", http.MethodPost, "comments", nil, request, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodeComment(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateComment(ctx context.Context, input UpdateCommentRequest, options ...socialhub.CallOption) (*Comment, error) {
	if err := c.requireUser("update_comment"); err != nil {
		return nil, err
	}
	if !validID(input.ID) || !validText(input.Content) {
		return nil, invalidArgument("update_comment", "comment ID or content is invalid")
	}
	request := mutationDocument{Data: mutationResource{
		Type: "comments", ID: input.ID, Attributes: map[string]any{"content": input.Content},
	}}
	var document resourceDocument
	if err := c.request(ctx, "update_comment", http.MethodPatch, "comments/"+url.PathEscape(input.ID), nil, request, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodeComment(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteComment(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if err := c.requireUser("delete_comment"); err != nil {
		return err
	}
	if !validID(id) {
		return invalidArgument("delete_comment", "comment ID is invalid")
	}
	return c.request(ctx, "delete_comment", http.MethodDelete, "comments/"+url.PathEscape(id), nil, nil, nil, options...)
}

func decodeComment(source resource, included map[string]resource) (Comment, error) {
	if source.Type != "comments" || !validID(source.ID) {
		return Comment{}, platformError("decode_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var result Comment
	if err := unmarshalAttributes(source, &result); err != nil {
		return Comment{}, err
	}
	result.ID = source.ID
	result.PostID = relationshipID(source, "post")
	result.UserID = relationshipID(source, "user")
	result.ParentID = relationshipID(source, "parent")
	if item, ok := included["users:"+result.UserID]; ok {
		user, err := decodeUser(item)
		if err != nil {
			return Comment{}, err
		}
		result.User = &user
	}
	return result, nil
}
