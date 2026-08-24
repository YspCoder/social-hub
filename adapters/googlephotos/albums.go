package googlephotos

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

// ListAlbums returns albums created by the same OAuth client application.
func (client *Client) ListAlbums(
	ctx context.Context,
	input ListAlbumsRequest,
	options ...socialhub.CallOption,
) (*Page[Album], error) {
	const operation = "list_albums"
	if !validPage(input.Page, 50) {
		return nil, invalidArgument(operation, "page size must be between 0 and 50 and page token must be opaque and bounded")
	}
	query := make(url.Values)
	setPageQuery(query, input.Page)
	meta, raw, err := client.doJSON(ctx, operation, http.MethodGet, "/v1/albums", query, nil, nil, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, "albums", 50, meta, raw, validAlbum, "album")
}

// GetAlbum returns one app-created album by persistent provider ID.
func (client *Client) GetAlbum(ctx context.Context, albumID string, options ...socialhub.CallOption) (*Album, ResponseMeta, error) {
	const operation = "get_album"
	if !validResourceID(albumID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "album ID must be a safe bounded path segment")
	}
	var album Album
	meta, _, err := client.doJSON(ctx, operation, http.MethodGet, "/v1/albums/"+albumID, nil, nil, &album, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validAlbum(album) || album.ID != albumID {
		return nil, meta, platformContractError(operation, "Google Photos returned an absent or mismatched album ID")
	}
	return &album, meta, nil
}

func setPageQuery(query url.Values, page PageOptions) {
	if page.PageSize > 0 {
		query.Set("pageSize", strconv.Itoa(page.PageSize))
	}
	if page.PageToken != "" {
		query.Set("pageToken", page.PageToken)
	}
}
