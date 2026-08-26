package tidal

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// ListTracks retrieves and cursor-pages track catalog resources.
func (client *Client) ListTracks(ctx context.Context, input ListTracksRequest, options ...socialhub.CallOption) (*Page[Track], error) {
	const operation = "list_tracks"
	if err := validateListTracksRequest(input); err != nil {
		return nil, err
	}
	query := make(url.Values)
	addArrayQuery(query, "filter[id]", input.IDs)
	addArrayQuery(query, "filter[isrc]", input.ISRCs)
	if input.Sort != "" {
		query.Set("sort", string(input.Sort))
	}
	if input.Cursor != "" {
		query.Set("page[cursor]", input.Cursor)
	}
	addCommonQuery(query, input.CountryCode, input.Include)
	body, response, err := client.get(ctx, operation, "/tracks", query, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, "/tracks", body, response, "tracks", func(resource *Track) (string, string) {
		return resource.Type, resource.ID
	})
}

// GetTrack retrieves one track by its opaque TIDAL identifier.
func (client *Client) GetTrack(ctx context.Context, id string, input ResourceRequest, options ...socialhub.CallOption) (*Document[Track], error) {
	const operation = "get_track"
	if !validOpaque(id, maxIDLength) {
		return nil, invalidArgument(operation, "track id is invalid")
	}
	if err := validateResourceRequest(operation, input, trackIncludeRoots); err != nil {
		return nil, err
	}
	query := make(url.Values)
	addCommonQuery(query, input.CountryCode, input.Include)
	body, response, err := client.get(ctx, operation, "/tracks/"+url.PathEscape(id), query, options...)
	if err != nil {
		return nil, err
	}
	return decodeDocument(operation, body, response, "tracks", func(resource *Track) (string, string) {
		return resource.Type, resource.ID
	})
}
