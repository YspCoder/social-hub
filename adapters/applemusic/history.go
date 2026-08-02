package applemusic

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

var historyTypes = map[HistoryResourceType]struct{}{
	HistoryArtists: {}, HistoryCurators: {}, HistoryAlbums: {}, HistoryLibraryAlbums: {},
	HistoryPlaylists: {}, HistoryLibraryPlaylists: {}, HistoryStations: {},
}

var historyTrackTypes = map[ResourceType]struct{}{
	ResourceSongs: {}, ResourceMusicVideos: {}, ResourceLibrarySongs: {}, ResourceLibraryMusicVideos: {},
}

func (c *Client) ListRecentlyPlayed(ctx context.Context, request RecentlyPlayedRequest, options ...socialhub.CallOption) (Page[AnyResource], error) {
	if err := c.requireMusicUserToken("list_recently_played"); err != nil {
		return Page[AnyResource]{}, err
	}
	if !validUniqueTypes(request.Types, historyTypes) || request.MaxResults < 0 || request.MaxResults > 10 || !validLanguage(request.Language) {
		return Page[AnyResource]{}, invalidArgument("list_recently_played", "resource types, limit, or language is invalid")
	}
	if _, ok := parseOffset(request.Cursor); !ok {
		return Page[AnyResource]{}, invalidArgument("list_recently_played", "cursor is invalid")
	}
	items := make([]string, len(request.Types))
	for index, value := range request.Types {
		items[index] = string(value)
	}
	return c.listHistory(ctx, "/me/recent/played", strings.Join(items, ","), request.Cursor, request.MaxResults, request.Language, options...)
}

func (c *Client) ListRecentlyPlayedTracks(ctx context.Context, request RecentlyPlayedTracksRequest, options ...socialhub.CallOption) (Page[AnyResource], error) {
	if err := c.requireMusicUserToken("list_recently_played_tracks"); err != nil {
		return Page[AnyResource]{}, err
	}
	if !validUniqueTypes(request.Types, historyTrackTypes) || request.MaxResults < 0 || request.MaxResults > 30 || !validLanguage(request.Language) {
		return Page[AnyResource]{}, invalidArgument("list_recently_played_tracks", "track types, limit, or language is invalid")
	}
	if _, ok := parseOffset(request.Cursor); !ok {
		return Page[AnyResource]{}, invalidArgument("list_recently_played_tracks", "cursor is invalid")
	}
	return c.listHistory(ctx, "/me/recent/played/tracks", joinResourceTypes(request.Types), request.Cursor, request.MaxResults, request.Language, options...)
}

func (c *Client) listHistory(ctx context.Context, path, types, cursor string, maxResults int, language string, options ...socialhub.CallOption) (Page[AnyResource], error) {
	query := url.Values{"types": {types}}
	if cursor != "" {
		query.Set("offset", cursor)
	}
	if maxResults > 0 {
		query.Set("limit", strconv.Itoa(maxResults))
	}
	if language != "" {
		query.Set("l", language)
	}
	var response apiCollection[AnyResource]
	if _, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return Page[AnyResource]{}, err
	}
	return toPage(response, path, c.apiBaseURL)
}

var _ HistoryWorkflow = (*Client)(nil)
