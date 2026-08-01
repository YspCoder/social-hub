package spotify

import (
	"encoding/json"
	"time"
)

type spotifyPage[T any] struct {
	Href     string `json:"href"`
	Items    []T    `json:"items"`
	Limit    int    `json:"limit"`
	Next     string `json:"next"`
	Offset   int    `json:"offset"`
	Previous string `json:"previous"`
	Total    int    `json:"total"`
}

type spotifyImage struct {
	URL    string `json:"url"`
	Height *int   `json:"height"`
	Width  *int   `json:"width"`
}

type spotifyArtist struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	URI          string            `json:"uri"`
	Href         string            `json:"href"`
	ExternalURLs map[string]string `json:"external_urls"`
}

type spotifyAlbum struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	URI                  string            `json:"uri"`
	Href                 string            `json:"href"`
	AlbumType            string            `json:"album_type"`
	ReleaseDate          string            `json:"release_date"`
	ReleaseDatePrecision string            `json:"release_date_precision"`
	TotalTracks          int               `json:"total_tracks"`
	Images               []spotifyImage    `json:"images"`
	Artists              []spotifyArtist   `json:"artists"`
	ExternalURLs         map[string]string `json:"external_urls"`
}

type spotifyTrack struct {
	Type         string            `json:"type"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	URI          string            `json:"uri"`
	Href         string            `json:"href"`
	DurationMS   int64             `json:"duration_ms"`
	Explicit     bool              `json:"explicit"`
	DiscNumber   int               `json:"disc_number"`
	TrackNumber  int               `json:"track_number"`
	IsPlayable   *bool             `json:"is_playable"`
	IsLocal      bool              `json:"is_local"`
	ExternalURLs map[string]string `json:"external_urls"`
	ExternalIDs  map[string]string `json:"external_ids"`
	Restrictions struct {
		Reason string `json:"reason"`
	} `json:"restrictions"`
	Album   spotifyAlbum    `json:"album"`
	Artists []spotifyArtist `json:"artists"`
}

type spotifyEpisode struct {
	Type                 string            `json:"type"`
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	URI                  string            `json:"uri"`
	Href                 string            `json:"href"`
	Description          string            `json:"description"`
	DurationMS           int64             `json:"duration_ms"`
	Explicit             bool              `json:"explicit"`
	IsExternallyHosted   bool              `json:"is_externally_hosted"`
	IsPlayable           *bool             `json:"is_playable"`
	ReleaseDate          string            `json:"release_date"`
	ReleaseDatePrecision string            `json:"release_date_precision"`
	Languages            []string          `json:"languages"`
	Images               []spotifyImage    `json:"images"`
	ExternalURLs         map[string]string `json:"external_urls"`
	Restrictions         struct {
		Reason string `json:"reason"`
	} `json:"restrictions"`
}

type spotifyPrivateUser struct {
	AccountID    string            `json:"account_id"`
	ID           string            `json:"id"`
	DisplayName  string            `json:"display_name"`
	URI          string            `json:"uri"`
	Href         string            `json:"href"`
	Product      string            `json:"product"`
	Country      string            `json:"country"`
	Email        string            `json:"email"`
	Images       []spotifyImage    `json:"images"`
	ExternalURLs map[string]string `json:"external_urls"`
	Followers    struct {
		Total int64 `json:"total"`
	} `json:"followers"`
}

type spotifyPlaylistOwner struct {
	AccountID    string            `json:"account_id"`
	ID           string            `json:"id"`
	DisplayName  string            `json:"display_name"`
	URI          string            `json:"uri"`
	ExternalURLs map[string]string `json:"external_urls"`
}

type spotifyPlaylist struct {
	ID            string                            `json:"id"`
	Name          string                            `json:"name"`
	URI           string                            `json:"uri"`
	Href          string                            `json:"href"`
	Description   string                            `json:"description"`
	Collaborative bool                              `json:"collaborative"`
	Public        *bool                             `json:"public"`
	SnapshotID    string                            `json:"snapshot_id"`
	Images        []spotifyImage                    `json:"images"`
	ExternalURLs  map[string]string                 `json:"external_urls"`
	Owner         spotifyPlaylistOwner              `json:"owner"`
	Items         *spotifyPage[spotifyPlaylistItem] `json:"items"`
}

type spotifyPlaylistItem struct {
	AddedAt *time.Time           `json:"added_at"`
	AddedBy spotifyPlaylistOwner `json:"added_by"`
	IsLocal bool                 `json:"is_local"`
	Item    json.RawMessage      `json:"item"`
}

type spotifySavedTrack struct {
	AddedAt *time.Time   `json:"added_at"`
	Track   spotifyTrack `json:"track"`
}

type spotifySearchResponse struct {
	Tracks spotifyPage[spotifyTrack] `json:"tracks"`
}

type spotifyDevice struct {
	ID               string `json:"id"`
	IsActive         bool   `json:"is_active"`
	IsPrivateSession bool   `json:"is_private_session"`
	IsRestricted     bool   `json:"is_restricted"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	VolumePercent    *int   `json:"volume_percent"`
	SupportsVolume   bool   `json:"supports_volume"`
}

type spotifyPlaybackState struct {
	Device               *spotifyDevice  `json:"device"`
	RepeatState          string          `json:"repeat_state"`
	ShuffleState         bool            `json:"shuffle_state"`
	Timestamp            int64           `json:"timestamp"`
	ProgressMS           *int64          `json:"progress_ms"`
	IsPlaying            bool            `json:"is_playing"`
	CurrentlyPlayingType string          `json:"currently_playing_type"`
	Item                 json.RawMessage `json:"item"`
	Context              *struct {
		Type string `json:"type"`
		URI  string `json:"uri"`
		Href string `json:"href"`
	} `json:"context"`
}

type spotifyDevicesResponse struct {
	Devices []spotifyDevice `json:"devices"`
}

type spotifyQueue struct {
	CurrentlyPlaying json.RawMessage   `json:"currently_playing"`
	Queue            []json.RawMessage `json:"queue"`
}

type spotifySnapshot struct {
	SnapshotID string `json:"snapshot_id"`
}

// Image is Spotify artwork metadata. Spotify policy requires original-form display and attribution.
type Image struct {
	URL    string `json:"url"`
	Height *int   `json:"height,omitempty"`
	Width  *int   `json:"width,omitempty"`
}

// Artist is the minimum current Spotify artist representation used by tracks and albums.
type Artist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URI         string `json:"uri"`
	ExternalURL string `json:"external_url,omitempty"`
}

// Album is the album subset embedded in Spotify track responses.
type Album struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	URI                  string   `json:"uri"`
	AlbumType            string   `json:"album_type,omitempty"`
	ReleaseDate          string   `json:"release_date,omitempty"`
	ReleaseDatePrecision string   `json:"release_date_precision,omitempty"`
	TotalTracks          int      `json:"total_tracks,omitempty"`
	ExternalURL          string   `json:"external_url,omitempty"`
	Images               []Image  `json:"images,omitempty"`
	Artists              []Artist `json:"artists,omitempty"`
}

// Track is Spotify catalog metadata. It never includes downloadable audio bytes.
type Track struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	URI         string            `json:"uri"`
	Duration    time.Duration     `json:"duration"`
	Explicit    bool              `json:"explicit"`
	DiscNumber  int               `json:"disc_number,omitempty"`
	TrackNumber int               `json:"track_number,omitempty"`
	IsPlayable  *bool             `json:"is_playable,omitempty"`
	IsLocal     bool              `json:"is_local"`
	Restriction string            `json:"restriction,omitempty"`
	ExternalURL string            `json:"external_url,omitempty"`
	ExternalIDs map[string]string `json:"external_ids,omitempty"`
	Album       Album             `json:"album"`
	Artists     []Artist          `json:"artists"`
}

// Episode is the podcast episode subset returned in playlists and playback.
type Episode struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	URI                  string        `json:"uri"`
	Description          string        `json:"description,omitempty"`
	Duration             time.Duration `json:"duration"`
	Explicit             bool          `json:"explicit"`
	IsExternallyHosted   bool          `json:"is_externally_hosted"`
	IsPlayable           *bool         `json:"is_playable,omitempty"`
	ReleaseDate          string        `json:"release_date,omitempty"`
	ReleaseDatePrecision string        `json:"release_date_precision,omitempty"`
	Restriction          string        `json:"restriction,omitempty"`
	ExternalURL          string        `json:"external_url,omitempty"`
	Languages            []string      `json:"languages,omitempty"`
	Images               []Image       `json:"images,omitempty"`
}

// Playable preserves the track-or-episode union used by playlist and player endpoints.
type Playable struct {
	Type    string          `json:"type"`
	Track   *Track          `json:"track,omitempty"`
	Episode *Episode        `json:"episode,omitempty"`
	Raw     json.RawMessage `json:"raw,omitempty"`
}

// SavedTrack is one entry in the current user's saved-track library.
type SavedTrack struct {
	AddedAt *time.Time `json:"added_at,omitempty"`
	Track   Track      `json:"track"`
}

// Playlist is current Spotify playlist metadata.
type Playlist struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	URI            string  `json:"uri"`
	Description    string  `json:"description,omitempty"`
	Collaborative  bool    `json:"collaborative"`
	Public         *bool   `json:"public,omitempty"`
	SnapshotID     string  `json:"snapshot_id,omitempty"`
	ExternalURL    string  `json:"external_url,omitempty"`
	OwnerID        string  `json:"owner_id,omitempty"`
	OwnerAccountID string  `json:"owner_account_id,omitempty"`
	Images         []Image `json:"images,omitempty"`
	ItemsTotal     int     `json:"items_total,omitempty"`
}

// PlaylistItem is one current /playlists/{id}/items entry.
type PlaylistItem struct {
	AddedAt *time.Time `json:"added_at,omitempty"`
	AddedBy string     `json:"added_by,omitempty"`
	IsLocal bool       `json:"is_local"`
	Item    Playable   `json:"item"`
}

// Device is a Spotify Connect playback device.
type Device struct {
	ID               string `json:"id"`
	IsActive         bool   `json:"is_active"`
	IsPrivateSession bool   `json:"is_private_session"`
	IsRestricted     bool   `json:"is_restricted"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	VolumePercent    *int   `json:"volume_percent,omitempty"`
	SupportsVolume   bool   `json:"supports_volume"`
}

// PlaybackState is the current Spotify Connect state.
type PlaybackState struct {
	Device               *Device        `json:"device,omitempty"`
	RepeatState          string         `json:"repeat_state,omitempty"`
	ShuffleState         bool           `json:"shuffle_state"`
	Timestamp            *time.Time     `json:"timestamp,omitempty"`
	Progress             *time.Duration `json:"progress,omitempty"`
	IsPlaying            bool           `json:"is_playing"`
	CurrentlyPlayingType string         `json:"currently_playing_type,omitempty"`
	ContextURI           string         `json:"context_uri,omitempty"`
	Item                 Playable       `json:"item"`
}

// Queue is the current item and upcoming Spotify playback queue.
type Queue struct {
	CurrentlyPlaying Playable   `json:"currently_playing"`
	Items            []Playable `json:"items"`
}
