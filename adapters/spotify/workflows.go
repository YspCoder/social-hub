package spotify

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	ScopeUserReadPrivate           = "user-read-private"
	ScopeUserReadEmail             = "user-read-email"
	ScopeUserLibraryRead           = "user-library-read"
	ScopeUserLibraryModify         = "user-library-modify"
	ScopeUserFollowRead            = "user-follow-read"
	ScopeUserFollowModify          = "user-follow-modify"
	ScopePlaylistReadPrivate       = "playlist-read-private"
	ScopePlaylistReadCollaborative = "playlist-read-collaborative"
	ScopePlaylistModifyPrivate     = "playlist-modify-private"
	ScopePlaylistModifyPublic      = "playlist-modify-public"
	ScopeUserReadPlaybackState     = "user-read-playback-state"
	ScopeUserReadCurrentlyPlaying  = "user-read-currently-playing"
	ScopeUserModifyPlaybackState   = "user-modify-playback-state"
)

// PaginationRequest uses Spotify's numeric offset as an opaque SDK cursor.
type PaginationRequest struct {
	Cursor     string
	MaxResults int
}

// SearchTracksRequest selects catalog tracks. Spotify currently caps each page at 10.
type SearchTracksRequest struct {
	Query      string
	Market     string
	Cursor     string
	MaxResults int
}

// SavedTracksRequest selects the current user's saved tracks.
type SavedTracksRequest struct {
	Market     string
	Cursor     string
	MaxResults int
}

// PlaylistItemsRequest selects one page of an owned or collaborative playlist.
type PlaylistItemsRequest struct {
	PlaylistID string
	Market     string
	Cursor     string
	MaxResults int
}

// CreatePlaylistRequest creates a playlist for the authenticated user.
type CreatePlaylistRequest struct {
	Name          string
	Description   string
	Public        *bool
	Collaborative bool
}

// ChangePlaylistDetailsRequest changes only the non-nil fields.
type ChangePlaylistDetailsRequest struct {
	PlaylistID    string
	Name          *string
	Description   *string
	Public        *bool
	Collaborative *bool
}

// AddPlaylistItemsRequest inserts up to 100 tracks or episodes.
type AddPlaylistItemsRequest struct {
	PlaylistID string
	URIs       []string
	Position   *int
}

// ReplacePlaylistItemsRequest replaces or clears all playlist items.
type ReplacePlaylistItemsRequest struct {
	PlaylistID string
	URIs       []string
}

// ReorderPlaylistItemsRequest moves a contiguous playlist range.
type ReorderPlaylistItemsRequest struct {
	PlaylistID   string
	RangeStart   int
	InsertBefore int
	RangeLength  int
	SnapshotID   string
}

// RemovePlaylistItemsRequest removes every occurrence of each supplied URI.
type RemovePlaylistItemsRequest struct {
	PlaylistID string
	URIs       []string
	SnapshotID string
}

// PlaybackOffset selects either a zero-based position or a playable URI.
type PlaybackOffset struct {
	Position *int
	URI      string
}

// StartPlaybackRequest resumes playback or starts one context/list.
type StartPlaybackRequest struct {
	DeviceID   string
	ContextURI string
	URIs       []string
	Offset     *PlaybackOffset
	Position   *time.Duration
}

// TransferPlaybackRequest transfers playback to one Spotify Connect device.
type TransferPlaybackRequest struct {
	DeviceID string
	Play     bool
}

// ProfileWorkflow reads the current authenticated Spotify profile.
type ProfileWorkflow interface {
	CurrentUser(context.Context, ...socialhub.CallOption) (*socialhub.User, error)
}

// CatalogWorkflow exposes the current track catalog endpoints retained by Spotify.
type CatalogWorkflow interface {
	GetTrack(context.Context, string, string, ...socialhub.CallOption) (*Track, error)
	SearchTracks(context.Context, SearchTracksRequest, ...socialhub.CallOption) (socialhub.Page[Track], error)
}

// LibraryWorkflow separates private collection state from social reactions.
type LibraryWorkflow interface {
	ListSavedTracks(context.Context, SavedTracksRequest, ...socialhub.CallOption) (socialhub.Page[SavedTrack], error)
	SaveItems(context.Context, []string, ...socialhub.CallOption) error
	RemoveItems(context.Context, []string, ...socialhub.CallOption) error
	ContainsItems(context.Context, []string, ...socialhub.CallOption) ([]bool, error)
}

// PlaylistWorkflow manages current-user playlists and their item snapshots.
type PlaylistWorkflow interface {
	ListCurrentUserPlaylists(context.Context, PaginationRequest, ...socialhub.CallOption) (socialhub.Page[Playlist], error)
	GetPlaylist(context.Context, string, string, ...socialhub.CallOption) (*Playlist, error)
	CreatePlaylist(context.Context, CreatePlaylistRequest, ...socialhub.CallOption) (*Playlist, error)
	ChangePlaylistDetails(context.Context, ChangePlaylistDetailsRequest, ...socialhub.CallOption) error
	ListPlaylistItems(context.Context, PlaylistItemsRequest, ...socialhub.CallOption) (socialhub.Page[PlaylistItem], error)
	AddPlaylistItems(context.Context, AddPlaylistItemsRequest, ...socialhub.CallOption) (string, error)
	ReplacePlaylistItems(context.Context, ReplacePlaylistItemsRequest, ...socialhub.CallOption) (string, error)
	ReorderPlaylistItems(context.Context, ReorderPlaylistItemsRequest, ...socialhub.CallOption) (string, error)
	RemovePlaylistItems(context.Context, RemovePlaylistItemsRequest, ...socialhub.CallOption) (string, error)
}

// PlaybackWorkflow exposes Spotify Connect state and Premium-only controls.
type PlaybackWorkflow interface {
	GetPlaybackState(context.Context, string, ...socialhub.CallOption) (*PlaybackState, error)
	ListDevices(context.Context, ...socialhub.CallOption) ([]Device, error)
	GetQueue(context.Context, ...socialhub.CallOption) (*Queue, error)
	TransferPlayback(context.Context, TransferPlaybackRequest, ...socialhub.CallOption) error
	StartPlayback(context.Context, StartPlaybackRequest, ...socialhub.CallOption) error
	PausePlayback(context.Context, string, ...socialhub.CallOption) error
	SkipNext(context.Context, string, ...socialhub.CallOption) error
	SkipPrevious(context.Context, string, ...socialhub.CallOption) error
	Seek(context.Context, time.Duration, string, ...socialhub.CallOption) error
	SetRepeat(context.Context, string, string, ...socialhub.CallOption) error
	SetVolume(context.Context, int, string, ...socialhub.CallOption) error
	SetShuffle(context.Context, bool, string, ...socialhub.CallOption) error
	AddToQueue(context.Context, string, string, ...socialhub.CallOption) error
}
