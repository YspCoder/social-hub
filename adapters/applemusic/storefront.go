package applemusic

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListStorefronts(ctx context.Context, options ...socialhub.CallOption) ([]Storefront, error) {
	var response apiCollection[Storefront]
	if _, err := c.requestJSON(ctx, http.MethodGet, "/storefronts", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) GetStorefront(ctx context.Context, storefront, language string, options ...socialhub.CallOption) (*Storefront, error) {
	if !validStorefront(storefront) || !validLanguage(language) {
		return nil, invalidArgument("get_storefront", "storefront or language is invalid")
	}
	query := url.Values{}
	if language != "" {
		query.Set("l", language)
	}
	return getResource[Storefront](ctx, c, "get_storefront", "/storefronts/"+url.PathEscape(strings.ToLower(storefront)), query, options...)
}

func (c *Client) CurrentUserStorefront(ctx context.Context, options ...socialhub.CallOption) (*Storefront, error) {
	if err := c.requireMusicUserToken("current_user_storefront"); err != nil {
		return nil, err
	}
	return getResource[Storefront](ctx, c, "current_user_storefront", "/me/storefront", nil, options...)
}

var _ StorefrontWorkflow = (*Client)(nil)
