package applemusic

import (
	"context"

	"social-hub/pkg/socialhub"
)

// PaginationRequest uses Apple's next-page offset as an opaque SDK cursor.
type PaginationRequest struct {
	Cursor     string
	MaxResults int
	Language   string
}

// CatalogSearchRequest selects Apple Music catalog resource types.
type CatalogSearchRequest struct {
	Storefront string
	Term       string
	Types      []ResourceType
	Cursor     string
	MaxResults int
	Language   string
}

// LibrarySearchRequest selects personal-library resource types.
type LibrarySearchRequest struct {
	Term       string
	Types      []ResourceType
	Cursor     string
	MaxResults int
	Language   string
}

// CatalogChartsRequest selects ranked catalog resources.
type CatalogChartsRequest struct {
	Storefront string
	Types      []ResourceType
	Chart      string
	Genre      string
	With       []string
	Cursor     string
	MaxResults int
	Language   string
}

// AddToLibraryRequest adds catalog identifiers to a user's library.
type AddToLibraryRequest struct {
	SongIDs       []string
	AlbumIDs      []string
	PlaylistIDs   []string
	MusicVideoIDs []string
	Language      string
}

// CreateLibraryPlaylistRequest creates a personal playlist with optional tracks.
type CreateLibraryPlaylistRequest struct {
	Name           string
	Description    string
	IsPublic       *bool
	Tracks         []ResourceReference
	ParentFolderID string
	Language       string
}

// AddPlaylistTracksRequest appends catalog or library tracks to a playlist.
type AddPlaylistTracksRequest struct {
	PlaylistID string
	Tracks     []ResourceReference
	Language   string
}

// HistoryResourceType identifies resource kinds accepted by recently played.
type HistoryResourceType string

const (
	HistoryArtists          HistoryResourceType = "artists"
	HistoryCurators         HistoryResourceType = "curators"
	HistoryAlbums           HistoryResourceType = "albums"
	HistoryLibraryAlbums    HistoryResourceType = "library-albums"
	HistoryPlaylists        HistoryResourceType = "playlists"
	HistoryLibraryPlaylists HistoryResourceType = "library-playlists"
	HistoryStations         HistoryResourceType = "stations"
)

// RecentlyPlayedRequest selects the non-track listening history endpoint.
type RecentlyPlayedRequest struct {
	Types      []HistoryResourceType
	Cursor     string
	MaxResults int
	Language   string
}

// RecentlyPlayedTracksRequest selects song and music-video history.
type RecentlyPlayedTracksRequest struct {
	Types      []ResourceType
	Cursor     string
	MaxResults int
	Language   string
}

type StorefrontWorkflow interface {
	ListStorefronts(context.Context, ...socialhub.CallOption) ([]Storefront, error)
	GetStorefront(context.Context, string, string, ...socialhub.CallOption) (*Storefront, error)
	CurrentUserStorefront(context.Context, ...socialhub.CallOption) (*Storefront, error)
}

type CatalogWorkflow interface {
	GetCatalogSong(context.Context, string, string, string, ...socialhub.CallOption) (*Song, error)
	GetCatalogAlbum(context.Context, string, string, string, ...socialhub.CallOption) (*Album, error)
	GetCatalogArtist(context.Context, string, string, string, ...socialhub.CallOption) (*Artist, error)
	GetCatalogPlaylist(context.Context, string, string, string, ...socialhub.CallOption) (*Playlist, error)
	GetCatalogMusicVideo(context.Context, string, string, string, ...socialhub.CallOption) (*MusicVideo, error)
	SearchCatalog(context.Context, CatalogSearchRequest, ...socialhub.CallOption) (*CatalogSearchResult, error)
	GetCatalogCharts(context.Context, CatalogChartsRequest, ...socialhub.CallOption) (*CatalogCharts, error)
}

type LibraryWorkflow interface {
	ListLibrarySongs(context.Context, PaginationRequest, ...socialhub.CallOption) (Page[Song], error)
	ListLibraryAlbums(context.Context, PaginationRequest, ...socialhub.CallOption) (Page[Album], error)
	ListLibraryArtists(context.Context, PaginationRequest, ...socialhub.CallOption) (Page[Artist], error)
	ListLibraryPlaylists(context.Context, PaginationRequest, ...socialhub.CallOption) (Page[Playlist], error)
	ListLibraryMusicVideos(context.Context, PaginationRequest, ...socialhub.CallOption) (Page[MusicVideo], error)
	SearchLibrary(context.Context, LibrarySearchRequest, ...socialhub.CallOption) (*LibrarySearchResult, error)
	AddToLibrary(context.Context, AddToLibraryRequest, ...socialhub.CallOption) error
}

type PlaylistWorkflow interface {
	CreateLibraryPlaylist(context.Context, CreateLibraryPlaylistRequest, ...socialhub.CallOption) (*Playlist, error)
	AddTracksToLibraryPlaylist(context.Context, AddPlaylistTracksRequest, ...socialhub.CallOption) error
}

type HistoryWorkflow interface {
	ListRecentlyPlayed(context.Context, RecentlyPlayedRequest, ...socialhub.CallOption) (Page[AnyResource], error)
	ListRecentlyPlayedTracks(context.Context, RecentlyPlayedTracksRequest, ...socialhub.CallOption) (Page[AnyResource], error)
}
