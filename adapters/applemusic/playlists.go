package applemusic

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

var playlistTrackTypes = map[ResourceType]struct{}{
	ResourceSongs: {}, ResourceMusicVideos: {}, ResourceLibrarySongs: {}, ResourceLibraryMusicVideos: {},
}

func (c *Client) CreateLibraryPlaylist(ctx context.Context, request CreateLibraryPlaylistRequest, options ...socialhub.CallOption) (*Playlist, error) {
	if err := c.requireMusicUserToken("create_library_playlist"); err != nil {
		return nil, err
	}
	if !validText(request.Name, false, 1024) || !validText(request.Description, true, 4096) || !validLanguage(request.Language) ||
		!validTrackReferences(request.Tracks) || (request.ParentFolderID != "" && !validIdentifier(request.ParentFolderID)) {
		return nil, invalidArgument("create_library_playlist", "playlist attributes or relationships are invalid")
	}
	type relation struct {
		Data []ResourceReference `json:"data"`
	}
	type relationships struct {
		Tracks *relation `json:"tracks,omitempty"`
		Parent *relation `json:"parent,omitempty"`
	}
	payload := struct {
		Attributes struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			IsPublic    *bool  `json:"isPublic,omitempty"`
		} `json:"attributes"`
		Relationships *relationships `json:"relationships,omitempty"`
	}{}
	payload.Attributes.Name = request.Name
	payload.Attributes.Description = request.Description
	payload.Attributes.IsPublic = request.IsPublic
	if len(request.Tracks) > 0 || request.ParentFolderID != "" {
		payload.Relationships = &relationships{}
		if len(request.Tracks) > 0 {
			payload.Relationships.Tracks = &relation{Data: request.Tracks}
		}
		if request.ParentFolderID != "" {
			payload.Relationships.Parent = &relation{Data: []ResourceReference{{ID: request.ParentFolderID, Type: ResourceLibraryPlaylistFolders}}}
		}
	}
	query := url.Values{}
	if request.Language != "" {
		query.Set("l", request.Language)
	}
	var response apiCollection[Playlist]
	if _, err := c.requestJSON(ctx, http.MethodPost, "/me/library/playlists", query, payload, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, platformError("create_library_playlist", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Data[0], nil
}

func (c *Client) AddTracksToLibraryPlaylist(ctx context.Context, request AddPlaylistTracksRequest, options ...socialhub.CallOption) error {
	if err := c.requireMusicUserToken("add_tracks_to_library_playlist"); err != nil {
		return err
	}
	if !validIdentifier(request.PlaylistID) || len(request.Tracks) == 0 || !validTrackReferences(request.Tracks) || !validLanguage(request.Language) {
		return invalidArgument("add_tracks_to_library_playlist", "playlist ID, tracks, or language is invalid")
	}
	query := url.Values{}
	if request.Language != "" {
		query.Set("l", request.Language)
	}
	payload := struct {
		Data []ResourceReference `json:"data"`
	}{Data: request.Tracks}
	path := "/me/library/playlists/" + url.PathEscape(request.PlaylistID) + "/tracks"
	_, err := c.requestJSON(ctx, http.MethodPost, path, query, payload, nil, options...)
	return err
}

func validTrackReferences(values []ResourceReference) bool {
	for _, value := range values {
		if !validIdentifier(value.ID) {
			return false
		}
		if _, ok := playlistTrackTypes[value.Type]; !ok {
			return false
		}
	}
	return true
}

var _ PlaylistWorkflow = (*Client)(nil)
