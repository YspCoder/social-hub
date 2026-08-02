package kitsu

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListPosts(ctx context.Context, input PostsRequest, options ...socialhub.CallOption) (socialhub.Page[Post], error) {
	offset, query, err := pagination(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[Post]{}, err
	}
	query.Set("sort", "-createdAt")
	query.Set("include", "user")
	var document collectionDocument
	if err := c.request(ctx, "list_posts", http.MethodGet, "posts", query, nil, &document, options...); err != nil {
		return socialhub.Page[Post]{}, err
	}
	index := includedIndex(document.Included)
	items := make([]Post, 0, len(document.Data))
	for _, item := range document.Data {
		decoded, err := decodePost(item, index)
		if err != nil {
			return socialhub.Page[Post]{}, err
		}
		items = append(items, decoded)
	}
	limit := input.Limit
	if limit == 0 {
		limit = maxPageSize
	}
	return toPage(items, document.Links, offset, limit)
}

func (c *Client) GetPost(ctx context.Context, id string, options ...socialhub.CallOption) (*Post, error) {
	if !validID(id) {
		return nil, invalidArgument("get_post", "post ID is invalid")
	}
	var document resourceDocument
	query := url.Values{"include": {"user"}}
	if err := c.request(ctx, "get_post", http.MethodGet, "posts/"+url.PathEscape(id), query, nil, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodePost(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreatePost(ctx context.Context, input CreatePostRequest, options ...socialhub.CallOption) (*Post, error) {
	if err := c.requireUser("create_post"); err != nil {
		return nil, err
	}
	if !validText(input.Content) || input.TargetGroupID != "" && !validID(input.TargetGroupID) ||
		input.TargetUserID != "" && !validID(input.TargetUserID) || input.MediaID != "" && !validID(input.MediaID) ||
		!validMediaKind(input.MediaKind) || input.MediaID != "" && input.MediaKind == "" || input.MediaID == "" && input.MediaKind != "" {
		return nil, invalidArgument("create_post", "content or relationship is invalid")
	}
	attributes := map[string]any{"content": input.Content, "spoiler": input.Spoiler, "nsfw": input.NSFW}
	relations := map[string]relationship{"user": identifierRelationship("users", c.userID)}
	if input.TargetGroupID != "" {
		relations["targetGroup"] = identifierRelationship("groups", input.TargetGroupID)
	}
	if input.TargetUserID != "" {
		relations["targetUser"] = identifierRelationship("users", input.TargetUserID)
	}
	if input.MediaID != "" {
		relations["media"] = identifierRelationship(strings.ToLower(string(input.MediaKind)), input.MediaID)
	}
	request := mutationDocument{Data: mutationResource{Type: "posts", Attributes: attributes, Relationships: relations}}
	var document resourceDocument
	if err := c.request(ctx, "create_post", http.MethodPost, "posts", nil, request, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodePost(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdatePost(ctx context.Context, input UpdatePostRequest, options ...socialhub.CallOption) (*Post, error) {
	if err := c.requireUser("update_post"); err != nil {
		return nil, err
	}
	if !validID(input.ID) || !validOptionalText(input.Content) || input.Content != nil && !validText(*input.Content) {
		return nil, invalidArgument("update_post", "post ID or content is invalid")
	}
	attributes := make(map[string]any)
	putPointer(attributes, "content", input.Content)
	putPointer(attributes, "spoiler", input.Spoiler)
	putPointer(attributes, "nsfw", input.NSFW)
	if len(attributes) == 0 {
		return nil, invalidArgument("update_post", "at least one field is required")
	}
	request := mutationDocument{Data: mutationResource{Type: "posts", ID: input.ID, Attributes: attributes}}
	var document resourceDocument
	if err := c.request(ctx, "update_post", http.MethodPatch, "posts/"+url.PathEscape(input.ID), nil, request, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodePost(document.Data, includedIndex(document.Included))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeletePost(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if err := c.requireUser("delete_post"); err != nil {
		return err
	}
	if !validID(id) {
		return invalidArgument("delete_post", "post ID is invalid")
	}
	return c.request(ctx, "delete_post", http.MethodDelete, "posts/"+url.PathEscape(id), nil, nil, nil, options...)
}

func decodePost(source resource, included map[string]resource) (Post, error) {
	if source.Type != "posts" || !validID(source.ID) {
		return Post{}, platformError("decode_post", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var result Post
	if err := unmarshalAttributes(source, &result); err != nil {
		return Post{}, err
	}
	result.ID, result.UserID = source.ID, relationshipID(source, "user")
	if item, ok := included["users:"+result.UserID]; ok {
		user, err := decodeUser(item)
		if err != nil {
			return Post{}, err
		}
		result.User = &user
	}
	return result, nil
}
