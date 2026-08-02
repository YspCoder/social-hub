package applemusic

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

var librarySearchTypes = map[ResourceType]struct{}{
	ResourceLibrarySongs: {}, ResourceLibraryAlbums: {}, ResourceLibraryArtists: {},
	ResourceLibraryPlaylists: {}, ResourceLibraryMusicVideos: {},
}

func (c *Client) ListLibrarySongs(ctx context.Context, request PaginationRequest, options ...socialhub.CallOption) (Page[Song], error) {
	return listLibrary[Song](ctx, c, "list_library_songs", "/me/library/songs", request, options...)
}

func (c *Client) ListLibraryAlbums(ctx context.Context, request PaginationRequest, options ...socialhub.CallOption) (Page[Album], error) {
	return listLibrary[Album](ctx, c, "list_library_albums", "/me/library/albums", request, options...)
}

func (c *Client) ListLibraryArtists(ctx context.Context, request PaginationRequest, options ...socialhub.CallOption) (Page[Artist], error) {
	return listLibrary[Artist](ctx, c, "list_library_artists", "/me/library/artists", request, options...)
}

func (c *Client) ListLibraryPlaylists(ctx context.Context, request PaginationRequest, options ...socialhub.CallOption) (Page[Playlist], error) {
	return listLibrary[Playlist](ctx, c, "list_library_playlists", "/me/library/playlists", request, options...)
}

func (c *Client) ListLibraryMusicVideos(ctx context.Context, request PaginationRequest, options ...socialhub.CallOption) (Page[MusicVideo], error) {
	return listLibrary[MusicVideo](ctx, c, "list_library_music_videos", "/me/library/music-videos", request, options...)
}

func listLibrary[T any](ctx context.Context, client *Client, operation, path string, request PaginationRequest, options ...socialhub.CallOption) (Page[T], error) {
	if err := client.requireMusicUserToken(operation); err != nil {
		return Page[T]{}, err
	}
	if _, ok := parseOffset(request.Cursor); !ok || request.MaxResults < 0 || !validLanguage(request.Language) {
		return Page[T]{}, invalidArgument(operation, "cursor, max results, or language is invalid")
	}
	query := url.Values{}
	addOptionalPageValues(query, request.Cursor, request.MaxResults, request.Language)
	var response apiCollection[T]
	if _, err := client.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return Page[T]{}, err
	}
	return toPage(response, path, client.apiBaseURL)
}

func (c *Client) SearchLibrary(ctx context.Context, request LibrarySearchRequest, options ...socialhub.CallOption) (*LibrarySearchResult, error) {
	if err := c.requireMusicUserToken("search_library"); err != nil {
		return nil, err
	}
	if !validText(request.Term, false, 256) || !validUniqueTypes(request.Types, librarySearchTypes) ||
		request.MaxResults < 0 || request.MaxResults > 25 || !validLanguage(request.Language) {
		return nil, invalidArgument("search_library", "term, resource types, limit, or language is invalid")
	}
	if _, ok := parseOffset(request.Cursor); !ok {
		return nil, invalidArgument("search_library", "cursor is invalid")
	}
	path := "/me/library/search"
	query := url.Values{"term": {request.Term}, "types": {joinResourceTypes(request.Types)}}
	addOptionalPageValues(query, request.Cursor, request.MaxResults, request.Language)
	var response struct {
		Results struct {
			Songs       apiCollection[Song]       `json:"library-songs"`
			Albums      apiCollection[Album]      `json:"library-albums"`
			Artists     apiCollection[Artist]     `json:"library-artists"`
			Playlists   apiCollection[Playlist]   `json:"library-playlists"`
			MusicVideos apiCollection[MusicVideo] `json:"library-music-videos"`
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
	return &LibrarySearchResult{Songs: songs, Albums: albums, Artists: artists, Playlists: playlists, MusicVideos: videos}, nil
}

func (c *Client) AddToLibrary(ctx context.Context, request AddToLibraryRequest, options ...socialhub.CallOption) error {
	if err := c.requireMusicUserToken("add_to_library"); err != nil {
		return err
	}
	groups := []struct {
		name string
		ids  []string
	}{
		{"songs", request.SongIDs}, {"albums", request.AlbumIDs},
		{"playlists", request.PlaylistIDs}, {"music-videos", request.MusicVideoIDs},
	}
	query := url.Values{}
	total := 0
	for _, group := range groups {
		if !validIDs(group.ids) {
			return invalidArgument("add_to_library", "resource IDs are invalid or duplicated")
		}
		if len(group.ids) > 0 {
			query.Set("ids["+group.name+"]", strings.Join(group.ids, ","))
			total += len(group.ids)
		}
	}
	if total == 0 || !validLanguage(request.Language) {
		return invalidArgument("add_to_library", "at least one resource ID and a valid language are required")
	}
	if request.Language != "" {
		query.Set("l", request.Language)
	}
	_, err := c.requestJSON(ctx, http.MethodPost, "/me/library", query, nil, nil, options...)
	return err
}

func validIDs(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validIdentifier(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

var _ LibraryWorkflow = (*Client)(nil)
