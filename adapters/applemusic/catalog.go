package applemusic

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

var catalogTypes = map[ResourceType]struct{}{
	ResourceSongs: {}, ResourceAlbums: {}, ResourceArtists: {}, ResourcePlaylists: {}, ResourceMusicVideos: {},
}

var chartTypes = map[ResourceType]struct{}{
	ResourceSongs: {}, ResourceAlbums: {}, ResourcePlaylists: {}, ResourceMusicVideos: {},
}

func (c *Client) GetCatalogSong(ctx context.Context, storefront, id, language string, options ...socialhub.CallOption) (*Song, error) {
	path, query, err := c.catalogResourceRequest("get_catalog_song", storefront, ResourceSongs, id, language)
	if err != nil {
		return nil, err
	}
	return getResource[Song](ctx, c, "get_catalog_song", path, query, options...)
}

func (c *Client) GetCatalogAlbum(ctx context.Context, storefront, id, language string, options ...socialhub.CallOption) (*Album, error) {
	path, query, err := c.catalogResourceRequest("get_catalog_album", storefront, ResourceAlbums, id, language)
	if err != nil {
		return nil, err
	}
	return getResource[Album](ctx, c, "get_catalog_album", path, query, options...)
}

func (c *Client) GetCatalogArtist(ctx context.Context, storefront, id, language string, options ...socialhub.CallOption) (*Artist, error) {
	path, query, err := c.catalogResourceRequest("get_catalog_artist", storefront, ResourceArtists, id, language)
	if err != nil {
		return nil, err
	}
	return getResource[Artist](ctx, c, "get_catalog_artist", path, query, options...)
}

func (c *Client) GetCatalogPlaylist(ctx context.Context, storefront, id, language string, options ...socialhub.CallOption) (*Playlist, error) {
	path, query, err := c.catalogResourceRequest("get_catalog_playlist", storefront, ResourcePlaylists, id, language)
	if err != nil {
		return nil, err
	}
	return getResource[Playlist](ctx, c, "get_catalog_playlist", path, query, options...)
}

func (c *Client) GetCatalogMusicVideo(ctx context.Context, storefront, id, language string, options ...socialhub.CallOption) (*MusicVideo, error) {
	path, query, err := c.catalogResourceRequest("get_catalog_music_video", storefront, ResourceMusicVideos, id, language)
	if err != nil {
		return nil, err
	}
	return getResource[MusicVideo](ctx, c, "get_catalog_music_video", path, query, options...)
}

func (c *Client) catalogResourceRequest(operation, requestedStorefront string, resourceType ResourceType, id, language string) (string, url.Values, error) {
	storefront, err := requestStorefront(requestedStorefront, c.storefront, operation)
	if err != nil {
		return "", nil, err
	}
	if !validIdentifier(id) || !validLanguage(language) {
		return "", nil, invalidArgument(operation, "resource ID or language is invalid")
	}
	query := url.Values{}
	if language != "" {
		query.Set("l", language)
	}
	path := "/catalog/" + url.PathEscape(storefront) + "/" + string(resourceType) + "/" + url.PathEscape(id)
	return path, query, nil
}

func (c *Client) SearchCatalog(ctx context.Context, request CatalogSearchRequest, options ...socialhub.CallOption) (*CatalogSearchResult, error) {
	storefront, err := requestStorefront(request.Storefront, c.storefront, "search_catalog")
	if err != nil {
		return nil, err
	}
	if !validText(request.Term, false, 256) || !validUniqueTypes(request.Types, catalogTypes) || request.MaxResults < 0 || request.MaxResults > 25 || !validLanguage(request.Language) {
		return nil, invalidArgument("search_catalog", "term, resource types, limit, or language is invalid")
	}
	if _, ok := parseOffset(request.Cursor); !ok {
		return nil, invalidArgument("search_catalog", "cursor is invalid")
	}
	path := "/catalog/" + url.PathEscape(storefront) + "/search"
	query := url.Values{"term": {request.Term}, "types": {joinResourceTypes(request.Types)}}
	addOptionalPageValues(query, request.Cursor, request.MaxResults, request.Language)
	var response struct {
		Results struct {
			Songs       apiCollection[Song]       `json:"songs"`
			Albums      apiCollection[Album]      `json:"albums"`
			Artists     apiCollection[Artist]     `json:"artists"`
			Playlists   apiCollection[Playlist]   `json:"playlists"`
			MusicVideos apiCollection[MusicVideo] `json:"music-videos"`
		} `json:"results"`
	}
	if _, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return nil, err
	}
	songs, err := toPage(response.Results.Songs, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	albums, err := toPage(response.Results.Albums, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	artists, err := toPage(response.Results.Artists, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	playlists, err := toPage(response.Results.Playlists, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	videos, err := toPage(response.Results.MusicVideos, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	return &CatalogSearchResult{Songs: songs, Albums: albums, Artists: artists, Playlists: playlists, MusicVideos: videos}, nil
}

func (c *Client) GetCatalogCharts(ctx context.Context, request CatalogChartsRequest, options ...socialhub.CallOption) (*CatalogCharts, error) {
	storefront, err := requestStorefront(request.Storefront, c.storefront, "get_catalog_charts")
	if err != nil {
		return nil, err
	}
	if !validUniqueTypes(request.Types, chartTypes) || request.MaxResults < 0 || request.MaxResults > 200 || !validLanguage(request.Language) ||
		!validOptionalValue(request.Chart, 128) || !validOptionalValue(request.Genre, 128) || !validChartModifiers(request.With) {
		return nil, invalidArgument("get_catalog_charts", "chart parameters are invalid")
	}
	if _, ok := parseOffset(request.Cursor); !ok {
		return nil, invalidArgument("get_catalog_charts", "cursor is invalid")
	}
	path := "/catalog/" + url.PathEscape(storefront) + "/charts"
	query := url.Values{"types": {joinResourceTypes(request.Types)}}
	addOptionalPageValues(query, request.Cursor, request.MaxResults, request.Language)
	if request.Chart != "" {
		query.Set("chart", request.Chart)
	}
	if request.Genre != "" {
		query.Set("genre", request.Genre)
	}
	if len(request.With) > 0 {
		query.Set("with", strings.Join(request.With, ","))
	}
	var response struct {
		Results struct {
			Songs       []apiChart[Song]       `json:"songs"`
			Albums      []apiChart[Album]      `json:"albums"`
			Playlists   []apiChart[Playlist]   `json:"playlists"`
			MusicVideos []apiChart[MusicVideo] `json:"music-videos"`
		} `json:"results"`
	}
	if _, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return nil, err
	}
	songs, err := convertCharts(response.Results.Songs, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	albums, err := convertCharts(response.Results.Albums, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	playlists, err := convertCharts(response.Results.Playlists, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	videos, err := convertCharts(response.Results.MusicVideos, path, c.apiBaseURL)
	if err != nil {
		return nil, err
	}
	return &CatalogCharts{Songs: songs, Albums: albums, Playlists: playlists, MusicVideos: videos}, nil
}

func convertCharts[T any](values []apiChart[T], path string, baseURL *url.URL) ([]Chart[T], error) {
	result := make([]Chart[T], 0, len(values))
	for _, value := range values {
		cursor, err := cursorFromNext(value.Next, path, baseURL)
		if err != nil {
			return nil, err
		}
		result = append(result, Chart[T]{Chart: value.Chart, Name: value.Name, OrderID: value.OrderID, Data: value.Data, NextCursor: cursor})
	}
	return result, nil
}

func joinResourceTypes(values []ResourceType) string {
	items := make([]string, len(values))
	for index, value := range values {
		items[index] = string(value)
	}
	return strings.Join(items, ",")
}

func addOptionalPageValues(query url.Values, cursor string, maxResults int, language string) {
	if cursor != "" {
		query.Set("offset", cursor)
	}
	if maxResults > 0 {
		query.Set("limit", strconv.Itoa(maxResults))
	}
	if language != "" {
		query.Set("l", language)
	}
}

func validOptionalValue(value string, maximum int) bool {
	return value == "" || validText(value, false, maximum)
}

func validChartModifiers(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "cityCharts" && value != "dailyGlobalTopCharts" {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

var _ CatalogWorkflow = (*Client)(nil)
