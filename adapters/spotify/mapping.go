package spotify

import (
	"bytes"
	"encoding/json"
	"maps"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) mapUser(input spotifyPrivateUser) (*socialhub.User, error) {
	stableID := firstNonEmpty(input.AccountID, c.spotifyAccountID)
	if !validSpotifyID(stableID) {
		return nil, mappingError("map_user", "Spotify profile did not contain a valid account_id")
	}
	if c.spotifyAccountID != "" && input.AccountID != "" && input.AccountID != c.spotifyAccountID {
		return nil, mappingError("map_user", "configured Spotify account_id did not match the profile")
	}
	extension, _ := json.Marshal(struct {
		SpotifyID string `json:"spotify_id,omitempty"`
		URI       string `json:"uri,omitempty"`
		Href      string `json:"href,omitempty"`
		Country   string `json:"country,omitempty"`
		Email     string `json:"email,omitempty"`
		Followers int64  `json:"followers"`
	}{input.ID, input.URI, input.Href, input.Country, input.Email, input.Followers.Total})
	var avatar string
	if len(input.Images) > 0 {
		avatar = input.Images[0].URL
	}
	return &socialhub.User{
		Platform: "spotify", AccountID: c.accountID, ID: stableID,
		Username: stringPointer(input.ID), DisplayName: stringPointer(input.DisplayName),
		AvatarURL: stringPointer(avatar), ProfileURL: stringPointer(input.ExternalURLs["spotify"]),
		AccountType: stringPointer(input.Product), Extensions: map[string]json.RawMessage{"spotify.user": extension},
	}, nil
}

func mapArtist(input spotifyArtist) (Artist, error) {
	if !validEntity(input.ID, input.URI, "artist") {
		return Artist{}, mappingError("map_artist", "Spotify returned an invalid artist identity")
	}
	return Artist{ID: input.ID, Name: input.Name, URI: input.URI, ExternalURL: input.ExternalURLs["spotify"]}, nil
}

func mapAlbum(input spotifyAlbum) (Album, error) {
	if !validEntity(input.ID, input.URI, "album") {
		return Album{}, mappingError("map_album", "Spotify returned an invalid album identity")
	}
	artists := make([]Artist, 0, len(input.Artists))
	for _, item := range input.Artists {
		artist, err := mapArtist(item)
		if err != nil {
			return Album{}, err
		}
		artists = append(artists, artist)
	}
	return Album{
		ID: input.ID, Name: input.Name, URI: input.URI, AlbumType: input.AlbumType,
		ReleaseDate: input.ReleaseDate, ReleaseDatePrecision: input.ReleaseDatePrecision,
		TotalTracks: input.TotalTracks, ExternalURL: input.ExternalURLs["spotify"],
		Images: mapImages(input.Images), Artists: artists,
	}, nil
}

func mapTrack(input spotifyTrack) (Track, error) {
	duration, validDuration := millisecondsDuration(input.DurationMS)
	if !validEntity(input.ID, input.URI, "track") || !validDuration {
		return Track{}, mappingError("map_track", "Spotify returned invalid track metadata")
	}
	album, err := mapAlbum(input.Album)
	if err != nil {
		return Track{}, err
	}
	artists := make([]Artist, 0, len(input.Artists))
	for _, item := range input.Artists {
		artist, err := mapArtist(item)
		if err != nil {
			return Track{}, err
		}
		artists = append(artists, artist)
	}
	return Track{
		ID: input.ID, Name: input.Name, URI: input.URI, Duration: duration,
		Explicit: input.Explicit, DiscNumber: input.DiscNumber, TrackNumber: input.TrackNumber,
		IsPlayable: input.IsPlayable, IsLocal: input.IsLocal, Restriction: input.Restrictions.Reason,
		ExternalURL: input.ExternalURLs["spotify"], ExternalIDs: maps.Clone(input.ExternalIDs),
		Album: album, Artists: artists,
	}, nil
}

func mapEpisode(input spotifyEpisode) (Episode, error) {
	duration, validDuration := millisecondsDuration(input.DurationMS)
	if !validEntity(input.ID, input.URI, "episode") || !validDuration {
		return Episode{}, mappingError("map_episode", "Spotify returned invalid episode metadata")
	}
	return Episode{
		ID: input.ID, Name: input.Name, URI: input.URI, Description: input.Description,
		Duration: duration, Explicit: input.Explicit,
		IsExternallyHosted: input.IsExternallyHosted, IsPlayable: input.IsPlayable,
		ReleaseDate: input.ReleaseDate, ReleaseDatePrecision: input.ReleaseDatePrecision,
		Restriction: input.Restrictions.Reason, ExternalURL: input.ExternalURLs["spotify"],
		Languages: append([]string(nil), input.Languages...), Images: mapImages(input.Images),
	}, nil
}

func mapPlayable(raw json.RawMessage) (Playable, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return Playable{}, nil
	}
	var discriminator struct {
		Type    string `json:"type"`
		IsLocal bool   `json:"is_local"`
	}
	if err := json.Unmarshal(trimmed, &discriminator); err != nil {
		return Playable{}, platformErrorWithCause("map_playable", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	copyRaw := append(json.RawMessage(nil), trimmed...)
	if discriminator.IsLocal {
		return Playable{Type: discriminator.Type, Raw: copyRaw}, nil
	}
	switch discriminator.Type {
	case "track":
		var input spotifyTrack
		if err := json.Unmarshal(trimmed, &input); err != nil {
			return Playable{}, platformErrorWithCause("map_playable", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		mapped, err := mapTrack(input)
		if err != nil {
			return Playable{}, err
		}
		return Playable{Type: "track", Track: &mapped}, nil
	case "episode":
		var input spotifyEpisode
		if err := json.Unmarshal(trimmed, &input); err != nil {
			return Playable{}, platformErrorWithCause("map_playable", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		mapped, err := mapEpisode(input)
		if err != nil {
			return Playable{}, err
		}
		return Playable{Type: "episode", Episode: &mapped}, nil
	default:
		return Playable{Type: discriminator.Type, Raw: copyRaw}, nil
	}
}

func mapPlaylist(input spotifyPlaylist) (Playlist, error) {
	if !validEntity(input.ID, input.URI, "playlist") {
		return Playlist{}, mappingError("map_playlist", "Spotify returned an invalid playlist identity")
	}
	ownerID := firstNonEmpty(input.Owner.AccountID, input.Owner.ID)
	itemsTotal := 0
	if input.Items != nil {
		itemsTotal = input.Items.Total
	}
	return Playlist{
		ID: input.ID, Name: input.Name, URI: input.URI, Description: input.Description,
		Collaborative: input.Collaborative, Public: input.Public, SnapshotID: input.SnapshotID,
		ExternalURL: input.ExternalURLs["spotify"], OwnerID: input.Owner.ID,
		OwnerAccountID: ownerID, Images: mapImages(input.Images), ItemsTotal: itemsTotal,
	}, nil
}

func mapPlaylistItem(input spotifyPlaylistItem) (PlaylistItem, error) {
	item, err := mapPlayable(input.Item)
	if err != nil {
		return PlaylistItem{}, err
	}
	return PlaylistItem{
		AddedAt: input.AddedAt, AddedBy: firstNonEmpty(input.AddedBy.AccountID, input.AddedBy.ID),
		IsLocal: input.IsLocal, Item: item,
	}, nil
}

func mapDevice(input spotifyDevice) Device {
	return Device{
		ID: input.ID, IsActive: input.IsActive, IsPrivateSession: input.IsPrivateSession,
		IsRestricted: input.IsRestricted, Name: input.Name, Type: input.Type,
		VolumePercent: input.VolumePercent, SupportsVolume: input.SupportsVolume,
	}
}

func mapPlaybackState(input spotifyPlaybackState) (*PlaybackState, error) {
	item, err := mapPlayable(input.Item)
	if err != nil {
		return nil, err
	}
	result := &PlaybackState{
		RepeatState: input.RepeatState, ShuffleState: input.ShuffleState, IsPlaying: input.IsPlaying,
		CurrentlyPlayingType: input.CurrentlyPlayingType, Item: item,
	}
	if input.Device != nil {
		mapped := mapDevice(*input.Device)
		result.Device = &mapped
	}
	if input.Timestamp > 0 {
		value := time.UnixMilli(input.Timestamp)
		result.Timestamp = &value
	}
	if input.ProgressMS != nil {
		value, valid := millisecondsDuration(*input.ProgressMS)
		if !valid {
			return nil, mappingError("map_playback", "Spotify returned invalid playback progress")
		}
		result.Progress = &value
	}
	if input.Context != nil {
		result.ContextURI = input.Context.URI
	}
	return result, nil
}

func mapImages(input []spotifyImage) []Image {
	images := make([]Image, 0, len(input))
	for _, item := range input {
		if strings.TrimSpace(item.URL) != "" {
			images = append(images, Image{URL: item.URL, Height: item.Height, Width: item.Width})
		}
	}
	return images
}

func validEntity(id, uri, kind string) bool {
	parts := strings.Split(uri, ":")
	return validSpotifyID(id) && len(parts) == 3 && parts[0] == "spotify" && parts[1] == kind && parts[2] == id
}

func millisecondsDuration(value int64) (time.Duration, bool) {
	maximum := int64(^uint64(0)>>1) / int64(time.Millisecond)
	if value < 0 || value > maximum {
		return 0, false
	}
	return time.Duration(value) * time.Millisecond, true
}

func mappingError(operation, message string) error { return platformError(operation, message) }

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}
