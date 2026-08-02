package kitsu

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) SearchAnime(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Media], error) {
	return c.searchMedia(ctx, "search_anime", "anime", MediaAnime, input, options...)
}

func (c *Client) SearchManga(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Media], error) {
	return c.searchMedia(ctx, "search_manga", "manga", MediaManga, input, options...)
}

func (c *Client) searchMedia(ctx context.Context, operation, path string, kind MediaKind, input SearchRequest, options ...socialhub.CallOption) (socialhub.Page[Media], error) {
	if !validSearch(input.Query) {
		return socialhub.Page[Media]{}, invalidArgument(operation, "query is invalid")
	}
	offset, query, err := pagination(input.Cursor, input.Limit)
	if err != nil {
		return socialhub.Page[Media]{}, err
	}
	query.Set("filter[text]", input.Query)
	var document collectionDocument
	if err := c.request(ctx, operation, http.MethodGet, path, query, nil, &document, options...); err != nil {
		return socialhub.Page[Media]{}, err
	}
	items := make([]Media, 0, len(document.Data))
	for _, item := range document.Data {
		decoded, err := decodeMedia(item, kind)
		if err != nil {
			return socialhub.Page[Media]{}, err
		}
		items = append(items, decoded)
	}
	limit := input.Limit
	if limit == 0 {
		limit = maxPageSize
	}
	return toPage(items, document.Links, offset, limit)
}

func (c *Client) GetAnime(ctx context.Context, id string, options ...socialhub.CallOption) (*Media, error) {
	return c.getMedia(ctx, "get_anime", "anime", MediaAnime, id, options...)
}

func (c *Client) GetManga(ctx context.Context, id string, options ...socialhub.CallOption) (*Media, error) {
	return c.getMedia(ctx, "get_manga", "manga", MediaManga, id, options...)
}

func (c *Client) getMedia(ctx context.Context, operation, path string, kind MediaKind, id string, options ...socialhub.CallOption) (*Media, error) {
	if !validID(id) {
		return nil, invalidArgument(operation, "media ID is invalid")
	}
	var document resourceDocument
	if err := c.request(ctx, operation, http.MethodGet, path+"/"+url.PathEscape(id), nil, nil, &document, options...); err != nil {
		return nil, err
	}
	decoded, err := decodeMedia(document.Data, kind)
	if err != nil {
		return nil, err
	}
	return &decoded, nil
}

func decodeMedia(source resource, kind MediaKind) (Media, error) {
	expected := "anime"
	if kind == MediaManga {
		expected = "manga"
	}
	if source.Type != expected || !validID(source.ID) {
		return Media{}, platformError("decode_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var result Media
	if err := unmarshalAttributes(source, &result); err != nil {
		return Media{}, err
	}
	result.ID, result.Kind = source.ID, kind
	return result, nil
}
