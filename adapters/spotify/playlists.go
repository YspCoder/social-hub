package spotify

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListCurrentUserPlaylists(ctx context.Context, input PaginationRequest, options ...socialhub.CallOption) (socialhub.Page[Playlist], error) {
	if err := c.requireScopes("list_playlists", ScopePlaylistReadPrivate); err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	query, err := pageQuery("list_playlists", input.Cursor, input.MaxResults, 50)
	if err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	var response spotifyPage[spotifyPlaylist]
	if _, err := c.requestJSON(ctx, http.MethodGet, "/me/playlists", query, nil, &response, options...); err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	items := make([]Playlist, 0, len(response.Items))
	for _, item := range response.Items {
		mapped, err := mapPlaylist(item)
		if err != nil {
			return socialhub.Page[Playlist]{}, err
		}
		items = append(items, mapped)
	}
	next, previous, err := c.pageCursors(response.Next, response.Previous)
	if err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	return socialhub.Page[Playlist]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (c *Client) GetPlaylist(ctx context.Context, playlistID, market string, options ...socialhub.CallOption) (*Playlist, error) {
	if err := c.requireScopes("get_playlist", ScopePlaylistReadPrivate); err != nil {
		return nil, err
	}
	if !validSpotifyID(playlistID) || !validMarket(market) {
		return nil, invalidArgument("get_playlist", "a valid playlist ID and optional uppercase market are required")
	}
	query := url.Values{"additional_types": {"track,episode"}}
	if market != "" {
		query.Set("market", market)
	}
	var response spotifyPlaylist
	if _, err := c.requestJSON(ctx, http.MethodGet, "/playlists/"+escapedID(playlistID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	mapped, err := mapPlaylist(response)
	if err != nil {
		return nil, err
	}
	if mapped.ID != playlistID {
		return nil, mappingError("get_playlist", "Spotify playlist response ID did not match the request")
	}
	return &mapped, nil
}

func (c *Client) CreatePlaylist(ctx context.Context, input CreatePlaylistRequest, options ...socialhub.CallOption) (*Playlist, error) {
	if err := validatePlaylistDetails("create_playlist", input.Name, input.Description); err != nil {
		return nil, err
	}
	if input.Collaborative && (input.Public == nil || *input.Public) {
		return nil, invalidArgument("create_playlist", "collaborative playlists must explicitly set public to false")
	}
	if input.Collaborative {
		if err := c.requireScopes("create_playlist", ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate); err != nil {
			return nil, err
		}
	} else if input.Public != nil && !*input.Public {
		if err := c.requireScopes("create_playlist", ScopePlaylistModifyPrivate); err != nil {
			return nil, err
		}
	} else if err := c.requireScopes("create_playlist", ScopePlaylistModifyPublic); err != nil {
		return nil, err
	}
	payload := struct {
		Name          string `json:"name"`
		Description   string `json:"description,omitempty"`
		Public        *bool  `json:"public,omitempty"`
		Collaborative bool   `json:"collaborative,omitempty"`
	}{input.Name, input.Description, input.Public, input.Collaborative}
	var response spotifyPlaylist
	if _, err := c.requestJSON(ctx, http.MethodPost, "/me/playlists", nil, payload, &response, options...); err != nil {
		return nil, err
	}
	mapped, err := mapPlaylist(response)
	if err != nil {
		return nil, err
	}
	return &mapped, nil
}

func (c *Client) ChangePlaylistDetails(ctx context.Context, input ChangePlaylistDetailsRequest, options ...socialhub.CallOption) error {
	if !validSpotifyID(input.PlaylistID) {
		return invalidArgument("change_playlist", "a valid Spotify playlist ID is required")
	}
	if input.Name == nil && input.Description == nil && input.Public == nil && input.Collaborative == nil {
		return invalidArgument("change_playlist", "at least one playlist field must be supplied")
	}
	if input.Name != nil && (strings.TrimSpace(*input.Name) == "" || utf8.RuneCountInString(*input.Name) > 100) {
		return invalidArgument("change_playlist", "playlist name must contain at most 100 characters")
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 300 {
		return invalidArgument("change_playlist", "playlist description must contain at most 300 characters")
	}
	if input.Collaborative != nil && *input.Collaborative && input.Public != nil && *input.Public {
		return invalidArgument("change_playlist", "a collaborative playlist cannot be public")
	}
	if input.Collaborative != nil && *input.Collaborative {
		if err := c.requireScopes("change_playlist", ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate); err != nil {
			return err
		}
	} else if input.Public != nil && !*input.Public {
		if err := c.requireScopes("change_playlist", ScopePlaylistModifyPrivate); err != nil {
			return err
		}
	} else if err := c.requireAnyScope("change_playlist", ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate); err != nil {
		return err
	}
	payload := struct {
		Name          *string `json:"name,omitempty"`
		Description   *string `json:"description,omitempty"`
		Public        *bool   `json:"public,omitempty"`
		Collaborative *bool   `json:"collaborative,omitempty"`
	}{input.Name, input.Description, input.Public, input.Collaborative}
	_, err := c.requestJSON(ctx, http.MethodPut, "/playlists/"+escapedID(input.PlaylistID), nil, payload, nil, options...)
	return err
}

func (c *Client) ListPlaylistItems(ctx context.Context, input PlaylistItemsRequest, options ...socialhub.CallOption) (socialhub.Page[PlaylistItem], error) {
	if err := c.requireScopes("list_playlist_items", ScopePlaylistReadPrivate); err != nil {
		return socialhub.Page[PlaylistItem]{}, err
	}
	if !validSpotifyID(input.PlaylistID) || !validMarket(input.Market) {
		return socialhub.Page[PlaylistItem]{}, invalidArgument("list_playlist_items", "a valid playlist ID and optional uppercase market are required")
	}
	query, err := pageQuery("list_playlist_items", input.Cursor, input.MaxResults, 50)
	if err != nil {
		return socialhub.Page[PlaylistItem]{}, err
	}
	query.Set("additional_types", "track,episode")
	if input.Market != "" {
		query.Set("market", input.Market)
	}
	var response spotifyPage[spotifyPlaylistItem]
	path := "/playlists/" + escapedID(input.PlaylistID) + "/items"
	if _, err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return socialhub.Page[PlaylistItem]{}, err
	}
	items := make([]PlaylistItem, 0, len(response.Items))
	for _, item := range response.Items {
		mapped, err := mapPlaylistItem(item)
		if err != nil {
			return socialhub.Page[PlaylistItem]{}, err
		}
		items = append(items, mapped)
	}
	next, previous, err := c.pageCursors(response.Next, response.Previous)
	if err != nil {
		return socialhub.Page[PlaylistItem]{}, err
	}
	return socialhub.Page[PlaylistItem]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (c *Client) AddPlaylistItems(ctx context.Context, input AddPlaylistItemsRequest, options ...socialhub.CallOption) (string, error) {
	if !validSpotifyID(input.PlaylistID) || !validPlayableURIs(input.URIs, false) {
		return "", invalidArgument("add_playlist_items", "a playlist ID and between 1 and 100 playable URIs are required")
	}
	if input.Position != nil && *input.Position < 0 {
		return "", invalidArgument("add_playlist_items", "position must not be negative")
	}
	if err := c.requireAnyScope("add_playlist_items", ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate); err != nil {
		return "", err
	}
	payload := struct {
		URIs     []string `json:"uris"`
		Position *int     `json:"position,omitempty"`
	}{input.URIs, input.Position}
	return c.playlistSnapshot(ctx, http.MethodPost, input.PlaylistID, payload, options...)
}

func (c *Client) ReplacePlaylistItems(ctx context.Context, input ReplacePlaylistItemsRequest, options ...socialhub.CallOption) (string, error) {
	if !validSpotifyID(input.PlaylistID) || !validPlayableURIs(input.URIs, true) {
		return "", invalidArgument("replace_playlist_items", "a playlist ID and at most 100 playable URIs are required")
	}
	if err := c.requireAnyScope("replace_playlist_items", ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate); err != nil {
		return "", err
	}
	payload := struct {
		URIs []string `json:"uris"`
	}{input.URIs}
	return c.playlistSnapshot(ctx, http.MethodPut, input.PlaylistID, payload, options...)
}

func (c *Client) ReorderPlaylistItems(ctx context.Context, input ReorderPlaylistItemsRequest, options ...socialhub.CallOption) (string, error) {
	if !validSpotifyID(input.PlaylistID) || input.RangeStart < 0 || input.InsertBefore < 0 || input.RangeLength < 0 || !validSnapshotID(input.SnapshotID) {
		return "", invalidArgument("reorder_playlist_items", "playlist ID, non-negative positions, and an optional valid snapshot ID are required")
	}
	if err := c.requireAnyScope("reorder_playlist_items", ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate); err != nil {
		return "", err
	}
	payload := struct {
		RangeStart   int    `json:"range_start"`
		InsertBefore int    `json:"insert_before"`
		RangeLength  int    `json:"range_length,omitempty"`
		SnapshotID   string `json:"snapshot_id,omitempty"`
	}{input.RangeStart, input.InsertBefore, input.RangeLength, input.SnapshotID}
	return c.playlistSnapshot(ctx, http.MethodPut, input.PlaylistID, payload, options...)
}

func (c *Client) RemovePlaylistItems(ctx context.Context, input RemovePlaylistItemsRequest, options ...socialhub.CallOption) (string, error) {
	if !validSpotifyID(input.PlaylistID) || !validPlayableURIs(input.URIs, false) || !validSnapshotID(input.SnapshotID) {
		return "", invalidArgument("remove_playlist_items", "playlist ID, 1-100 playable URIs, and an optional valid snapshot ID are required")
	}
	if err := c.requireAnyScope("remove_playlist_items", ScopePlaylistModifyPublic, ScopePlaylistModifyPrivate); err != nil {
		return "", err
	}
	items := make([]struct {
		URI string `json:"uri"`
	}, len(input.URIs))
	for index, uri := range input.URIs {
		items[index].URI = uri
	}
	payload := struct {
		Items []struct {
			URI string `json:"uri"`
		} `json:"items"`
		SnapshotID string `json:"snapshot_id,omitempty"`
	}{items, input.SnapshotID}
	return c.playlistSnapshot(ctx, http.MethodDelete, input.PlaylistID, payload, options...)
}

func (c *Client) playlistSnapshot(ctx context.Context, method, playlistID string, payload any, options ...socialhub.CallOption) (string, error) {
	var response spotifySnapshot
	path := "/playlists/" + escapedID(playlistID) + "/items"
	if _, err := c.requestJSON(ctx, method, path, nil, payload, &response, options...); err != nil {
		return "", err
	}
	if !validSnapshotID(response.SnapshotID) || response.SnapshotID == "" {
		return "", mappingError("playlist_snapshot", "Spotify did not return a valid playlist snapshot ID")
	}
	return response.SnapshotID, nil
}

func (c *Client) pageCursors(nextURL, previousURL string) (*string, *string, error) {
	next, err := pageCursor(nextURL, c.apiBaseURL)
	if err != nil {
		return nil, nil, err
	}
	previous, err := pageCursor(previousURL, c.apiBaseURL)
	if err != nil {
		return nil, nil, err
	}
	return next, previous, nil
}

func validatePlaylistDetails(operation, name, description string) error {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 100 {
		return invalidArgument(operation, "playlist name must contain at most 100 characters")
	}
	if utf8.RuneCountInString(description) > 300 {
		return invalidArgument(operation, "playlist description must contain at most 300 characters")
	}
	return nil
}

func validPlayableURIs(uris []string, allowEmpty bool) bool {
	if len(uris) > 100 || (!allowEmpty && len(uris) == 0) {
		return false
	}
	for _, uri := range uris {
		if !validPlayableURI(uri) {
			return false
		}
	}
	return true
}

func validSnapshotID(value string) bool {
	return value == "" || (len(value) <= 1_024 && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl))
}

var _ PlaylistWorkflow = (*Client)(nil)
