package dailymotion

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	playlistFields      = "playlist_id,title,description,visibility,created_at,updated_at,profile,playlist_url,embed_url"
	playlistVideoFields = "title,description,created_at"
)

func (c *Client) GetPlaylist(ctx context.Context, playlistID string, options ...socialhub.CallOption) (*Playlist, error) {
	if err := c.requireScopes("get_playlist", ScopePlaylistRead); err != nil {
		return nil, err
	}
	if !validResourceID(playlistID) {
		return nil, invalidArgument("get_playlist", "a valid playlist ID is required")
	}
	var response Playlist
	if err := c.requestJSON(ctx, http.MethodGet, "/playlists/"+escapedID(playlistID), url.Values{"fields": {playlistFields}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.PlaylistID != playlistID {
		return nil, platformError("get_playlist", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) ListPlaylists(ctx context.Context, input PlaylistListRequest, options ...socialhub.CallOption) (socialhub.Page[Playlist], error) {
	if err := c.requireScopes("list_playlists", ScopePlaylistRead); err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	profileID, err := c.defaultProfile("list_playlists", input.ProfileID)
	if err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	query, err := pageQuery("list_playlists", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	if !validSort(input.Sort, map[string]struct{}{"created_at": {}}) || input.CreatedAfter != nil && input.CreatedBefore != nil && input.CreatedAfter.After(*input.CreatedBefore) {
		return socialhub.Page[Playlist]{}, invalidArgument("list_playlists", "sort or time range is invalid")
	}
	query.Set("fields", playlistFields)
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	if input.CreatedAfter != nil {
		query.Set("created_after", input.CreatedAfter.UTC().Format(timeFormat))
	}
	if input.CreatedBefore != nil {
		query.Set("created_before", input.CreatedBefore.UTC().Format(timeFormat))
	}
	var response apiPage[Playlist]
	if err := c.requestJSON(ctx, http.MethodGet, "/profiles/"+escapedID(profileID)+"/playlists", query, nil, &response, options...); err != nil {
		return socialhub.Page[Playlist]{}, err
	}
	return mapPage(c, response)
}

func (c *Client) CreatePlaylist(ctx context.Context, input CreatePlaylistRequest, options ...socialhub.CallOption) (*Playlist, error) {
	if err := c.requireScopes("create_playlist", ScopePlaylistManage); err != nil {
		return nil, err
	}
	profileID, err := c.defaultProfile("create_playlist", input.ProfileID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Title) == "" || utf8.RuneCountInString(input.Title) > 50 || utf8.RuneCountInString(input.Description) > 2000 || !validPlaylistVisibility(input.Visibility) {
		return nil, invalidArgument("create_playlist", "title, description, or visibility is invalid")
	}
	body := struct {
		Title       string `json:"title"`
		Description string `json:"description,omitempty"`
		Visibility  string `json:"visibility"`
	}{input.Title, input.Description, input.Visibility}
	var response Playlist
	if err := c.requestJSON(ctx, http.MethodPost, "/profiles/"+escapedID(profileID)+"/playlists", nil, body, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.PlaylistID) {
		return nil, platformError("create_playlist", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (c *Client) UpdatePlaylist(ctx context.Context, playlistID string, input UpdatePlaylistRequest, options ...socialhub.CallOption) error {
	if err := c.requireScopes("update_playlist", ScopePlaylistManage); err != nil {
		return err
	}
	if !validResourceID(playlistID) {
		return invalidArgument("update_playlist", "a valid playlist ID is required")
	}
	if input.Title == nil && input.Description == nil && input.Visibility == nil {
		return invalidArgument("update_playlist", "at least one mutable field is required")
	}
	if input.Title != nil && (strings.TrimSpace(*input.Title) == "" || utf8.RuneCountInString(*input.Title) > 50) || input.Description != nil && utf8.RuneCountInString(*input.Description) > 2000 || input.Visibility != nil && !validPlaylistVisibility(*input.Visibility) {
		return invalidArgument("update_playlist", "title, description, or visibility is invalid")
	}
	body := struct {
		Title       *string `json:"title,omitempty"`
		Description *string `json:"description,omitempty"`
		Visibility  *string `json:"visibility,omitempty"`
	}{input.Title, input.Description, input.Visibility}
	return c.requestJSON(ctx, http.MethodPatch, "/playlists/"+escapedID(playlistID), nil, body, nil, options...)
}

func (c *Client) DeletePlaylist(ctx context.Context, playlistID string, options ...socialhub.CallOption) error {
	if err := c.requireScopes("delete_playlist", ScopePlaylistManage); err != nil {
		return err
	}
	if !validResourceID(playlistID) {
		return invalidArgument("delete_playlist", "a valid playlist ID is required")
	}
	return c.requestJSON(ctx, http.MethodDelete, "/playlists/"+escapedID(playlistID), nil, nil, nil, options...)
}

func (c *Client) ListPlaylistVideos(ctx context.Context, input PlaylistVideosRequest, options ...socialhub.CallOption) (socialhub.Page[PlaylistVideo], error) {
	if err := c.requireScopes("list_playlist_videos", ScopePlaylistRead); err != nil {
		return socialhub.Page[PlaylistVideo]{}, err
	}
	if !validResourceID(input.PlaylistID) || !validSort(input.Sort, map[string]struct{}{"created_at": {}}) {
		return socialhub.Page[PlaylistVideo]{}, invalidArgument("list_playlist_videos", "a valid playlist ID and sort are required")
	}
	query, err := pageQuery("list_playlist_videos", input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[PlaylistVideo]{}, err
	}
	query.Set("fields", playlistVideoFields)
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	var response apiPage[PlaylistVideo]
	if err := c.requestJSON(ctx, http.MethodGet, "/playlists/"+escapedID(input.PlaylistID)+"/videos", query, nil, &response, options...); err != nil {
		return socialhub.Page[PlaylistVideo]{}, err
	}
	return mapPage(c, response)
}

func (c *Client) AddPlaylistVideo(ctx context.Context, playlistID, videoID, beforeVideoID string, options ...socialhub.CallOption) (*PlaylistVideo, error) {
	if err := c.requireScopes("add_playlist_video", ScopePlaylistManage); err != nil {
		return nil, err
	}
	if !validResourceID(playlistID) || !validResourceID(videoID) || beforeVideoID != "" && !validResourceID(beforeVideoID) {
		return nil, invalidArgument("add_playlist_video", "valid playlist and video IDs are required")
	}
	body := struct {
		VideoID       string `json:"video_id"`
		BeforeVideoID string `json:"before_video_id,omitempty"`
	}{videoID, beforeVideoID}
	var response PlaylistVideo
	if err := c.requestJSON(ctx, http.MethodPost, "/playlists/"+escapedID(playlistID)+"/videos", nil, body, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) RemovePlaylistVideo(ctx context.Context, playlistID, videoID string, options ...socialhub.CallOption) error {
	if err := c.requireScopes("remove_playlist_video", ScopePlaylistManage); err != nil {
		return err
	}
	if !validResourceID(playlistID) || !validResourceID(videoID) {
		return invalidArgument("remove_playlist_video", "valid playlist and video IDs are required")
	}
	return c.requestJSON(ctx, http.MethodDelete, "/playlists/"+escapedID(playlistID)+"/videos/"+escapedID(videoID), nil, nil, nil, options...)
}

var _ PlaylistWorkflow = (*Client)(nil)
