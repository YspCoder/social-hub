package kitsu

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetUser(ctx context.Context, id string, options ...socialhub.CallOption) (*User, error) {
	if !validID(id) {
		return nil, invalidArgument("get_user", "user ID is invalid")
	}
	var document resourceDocument
	if err := c.request(ctx, "get_user", http.MethodGet, "users/"+url.PathEscape(id), nil, nil, &document, options...); err != nil {
		return nil, err
	}
	result, err := decodeUser(document.Data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) FindUserBySlug(ctx context.Context, slug string, options ...socialhub.CallOption) (*User, error) {
	if !validSlug(slug) {
		return nil, invalidArgument("find_user_by_slug", "slug is invalid")
	}
	return c.findUser(ctx, "find_user_by_slug", url.Values{"filter[slug]": {slug}, "page[limit]": {"1"}}, options...)
}

func (c *Client) GetCurrentUser(ctx context.Context, options ...socialhub.CallOption) (*User, error) {
	if err := c.requireToken("get_current_user"); err != nil {
		return nil, err
	}
	return c.findUser(ctx, "get_current_user", url.Values{"filter[self]": {"true"}, "page[limit]": {"1"}}, options...)
}

func (c *Client) findUser(ctx context.Context, operation string, query url.Values, options ...socialhub.CallOption) (*User, error) {
	var document collectionDocument
	if err := c.request(ctx, operation, http.MethodGet, "users", query, nil, &document, options...); err != nil {
		return nil, err
	}
	if len(document.Data) == 0 {
		return nil, platformError(operation, socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	result, err := decodeUser(document.Data[0])
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func decodeUser(source resource) (User, error) {
	if source.Type != "users" || !validID(source.ID) {
		return User{}, platformError("decode_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var result User
	if err := unmarshalAttributes(source, &result); err != nil {
		return User{}, err
	}
	result.ID = source.ID
	return result, nil
}
