package listenbrainz

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type playlistEnvelope struct {
	Playlist Playlist `json:"playlist"`
}

type playlistPageEnvelope struct {
	Count         int                `json:"count"`
	Offset        int                `json:"offset"`
	PlaylistCount int                `json:"playlist_count"`
	Playlists     []playlistEnvelope `json:"playlists"`
}

func (c *Client) SearchPlaylists(ctx context.Context, request PlaylistSearchRequest, options ...socialhub.CallOption) (socialhub.Page[Playlist], error) {
	const operation = "search_playlists"
	if !validText(request.Query, maxTextLength) || len(request.Query) < 3 {
		return socialhub.Page[Playlist]{}, invalidArgument(operation, "query must contain at least 3 characters")
	}
	query, offset, err := playlistPageQuery(request.Cursor, request.MaxResults)
	if err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	query.Set("query", request.Query)
	return c.getPlaylistPage(ctx, operation, "/1/playlist/search", query, offset, request.MaxResults, options...)
}

func (c *Client) ListUserPlaylists(ctx context.Context, requestedUsername string, request PlaylistPageRequest, options ...socialhub.CallOption) (socialhub.Page[Playlist], error) {
	const operation = "list_user_playlists"
	username, err := c.resolveUsername(operation, requestedUsername)
	if err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	query, offset, err := playlistPageQuery(request.Cursor, request.MaxResults)
	if err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	path := "/1/user/" + url.PathEscape(username) + "/playlists"
	return c.getPlaylistPage(ctx, operation, path, query, offset, request.MaxResults, options...)
}

func playlistPageQuery(cursor string, maxResults int) (url.Values, int, error) {
	if err := validatePage(maxResults, maxPlaylistPageSize); err != nil {
		return nil, 0, err
	}
	offset, err := validateOffset(cursor, maxPlaylistPageSize)
	if err != nil {
		return nil, 0, err
	}
	query := make(url.Values)
	if maxResults > 0 {
		query.Set("count", strconv.Itoa(maxResults))
	}
	if cursor != "" {
		query.Set("offset", cursor)
	}
	return query, offset, nil
}

func (c *Client) getPlaylistPage(ctx context.Context, operation, path string, query url.Values, expectedOffset, requestedLimit int, options ...socialhub.CallOption) (socialhub.Page[Playlist], error) {
	var envelope playlistPageEnvelope
	if err := getOnly(ctx, c, operation, path, query, &envelope, options...); err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	if envelope.Count > maxPlaylistPageSize {
		return socialhub.Page[Playlist]{}, invalidPlatformResponse(operation, "response exceeded the playlist page limit")
	}
	items := make([]Playlist, len(envelope.Playlists))
	for index, item := range envelope.Playlists {
		items[index] = item.Playlist
	}
	return offsetPage(operation, items, envelope.Count, envelope.Offset, envelope.PlaylistCount, expectedOffset, requestedLimit)
}

func (c *Client) GetPlaylist(ctx context.Context, playlistMBID string, fetchMetadata bool, options ...socialhub.CallOption) (*Playlist, error) {
	const operation = "get_playlist"
	if !validMBID(playlistMBID) {
		return nil, invalidArgument(operation, "playlist_mbid must be a canonical lowercase UUID")
	}
	query := url.Values{"fetch_metadata": {strconv.FormatBool(fetchMetadata)}}
	var envelope playlistEnvelope
	if err := getOnly(ctx, c, operation, "/1/playlist/"+playlistMBID, query, &envelope, options...); err != nil {
		return nil, err
	}
	if envelope.Playlist.Title == "" {
		return nil, invalidPlatformResponse(operation, "response omitted playlist title")
	}
	return &envelope.Playlist, nil
}
