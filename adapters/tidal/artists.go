package tidal

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// ListArtists retrieves up to 20 artists selected by IDs or handles.
func (client *Client) ListArtists(ctx context.Context, input ListArtistsRequest, options ...socialhub.CallOption) (*Page[Artist], error) {
	const operation = "list_artists"
	if err := validateListArtistsRequest(input); err != nil {
		return nil, err
	}
	query := make(url.Values)
	addArrayQuery(query, "filter[id]", input.IDs)
	addArrayQuery(query, "filter[handle]", input.Handles)
	addCommonQuery(query, input.CountryCode, input.Include)
	body, response, err := client.get(ctx, operation, "/artists", query, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, "/artists", body, response, "artists", func(resource *Artist) (string, string) {
		return resource.Type, resource.ID
	})
}

// GetArtist retrieves one artist by its opaque TIDAL identifier.
func (client *Client) GetArtist(ctx context.Context, id string, input ResourceRequest, options ...socialhub.CallOption) (*Document[Artist], error) {
	const operation = "get_artist"
	if !validOpaque(id, maxIDLength) {
		return nil, invalidArgument(operation, "artist id is invalid")
	}
	if err := validateResourceRequest(operation, input, artistIncludeRoots); err != nil {
		return nil, err
	}
	query := make(url.Values)
	addCommonQuery(query, input.CountryCode, input.Include)
	body, response, err := client.get(ctx, operation, "/artists/"+url.PathEscape(id), query, options...)
	if err != nil {
		return nil, err
	}
	return decodeDocument(operation, body, response, "artists", func(resource *Artist) (string, string) {
		return resource.Type, resource.ID
	})
}
