package tidal

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// ListAlbums retrieves and cursor-pages album catalog resources.
func (client *Client) ListAlbums(ctx context.Context, input ListAlbumsRequest, options ...socialhub.CallOption) (*Page[Album], error) {
	const operation = "list_albums"
	if err := validateListAlbumsRequest(input); err != nil {
		return nil, err
	}
	query := make(url.Values)
	addArrayQuery(query, "filter[id]", input.IDs)
	addArrayQuery(query, "filter[barcodeId]", input.BarcodeIDs)
	if input.Sort != "" {
		query.Set("sort", string(input.Sort))
	}
	if input.Cursor != "" {
		query.Set("page[cursor]", input.Cursor)
	}
	addCommonQuery(query, input.CountryCode, input.Include)
	body, response, err := client.get(ctx, operation, "/albums", query, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, "/albums", body, response, "albums", func(resource *Album) (string, string) {
		return resource.Type, resource.ID
	})
}

// GetAlbum retrieves one album by its opaque TIDAL identifier.
func (client *Client) GetAlbum(ctx context.Context, id string, input ResourceRequest, options ...socialhub.CallOption) (*Document[Album], error) {
	const operation = "get_album"
	if !validOpaque(id, maxIDLength) {
		return nil, invalidArgument(operation, "album id is invalid")
	}
	if err := validateResourceRequest(operation, input, albumIncludeRoots); err != nil {
		return nil, err
	}
	query := make(url.Values)
	addCommonQuery(query, input.CountryCode, input.Include)
	body, response, err := client.get(ctx, operation, "/albums/"+url.PathEscape(id), query, options...)
	if err != nil {
		return nil, err
	}
	return decodeDocument(operation, body, response, "albums", func(resource *Album) (string, string) {
		return resource.Type, resource.ID
	})
}
