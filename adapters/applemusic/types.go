package applemusic

import "encoding/json"

// ResourceType identifies an Apple Music catalog or library resource.
type ResourceType string

const (
	ResourceAlbums                 ResourceType = "albums"
	ResourceArtists                ResourceType = "artists"
	ResourceMusicVideos            ResourceType = "music-videos"
	ResourcePlaylists              ResourceType = "playlists"
	ResourceSongs                  ResourceType = "songs"
	ResourceLibraryAlbums          ResourceType = "library-albums"
	ResourceLibraryArtists         ResourceType = "library-artists"
	ResourceLibraryMusicVideos     ResourceType = "library-music-videos"
	ResourceLibraryPlaylistFolders ResourceType = "library-playlist-folders"
	ResourceLibraryPlaylists       ResourceType = "library-playlists"
	ResourceLibrarySongs           ResourceType = "library-songs"
)

// ResourceReference identifies a resource in a request or relationship.
type ResourceReference struct {
	ID   string       `json:"id"`
	Type ResourceType `json:"type"`
}

// Relationship contains identifiers and an optional relative next-page link.
type Relationship struct {
	Href string              `json:"href,omitempty"`
	Next string              `json:"next,omitempty"`
	Data []ResourceReference `json:"data,omitempty"`
}

// Artwork describes an Apple artwork URL template and its color metadata.
type Artwork struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	URL        string `json:"url"`
	Background string `json:"bgColor,omitempty"`
	TextColor1 string `json:"textColor1,omitempty"`
	TextColor2 string `json:"textColor2,omitempty"`
	TextColor3 string `json:"textColor3,omitempty"`
	TextColor4 string `json:"textColor4,omitempty"`
}

// EditorialNotes contains localized editorial copy.
type EditorialNotes struct {
	Name     string `json:"name,omitempty"`
	Short    string `json:"short,omitempty"`
	Standard string `json:"standard,omitempty"`
	Tagline  string `json:"tagline,omitempty"`
}

// DescriptionAttribute contains short and standard descriptions.
type DescriptionAttribute struct {
	Short    string `json:"short,omitempty"`
	Standard string `json:"standard,omitempty"`
}

// PlayParameters identifies content for MusicKit playback. It is metadata only;
// this adapter does not retrieve protected audio.
type PlayParameters struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	IsLibrary   bool   `json:"isLibrary,omitempty"`
	CatalogID   string `json:"catalogId,omitempty"`
	ReportingID string `json:"reportingId,omitempty"`
}

// Preview identifies an Apple-provided preview URL when present.
type Preview struct {
	URL    string `json:"url"`
	HLSURL string `json:"hlsUrl,omitempty"`
}

// Storefront is an Apple Music geographic and localization boundary.
type Storefront struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Href       string               `json:"href,omitempty"`
	Attributes StorefrontAttributes `json:"attributes"`
}

type StorefrontAttributes struct {
	DefaultLanguageTag    string   `json:"defaultLanguageTag"`
	ExplicitContentPolicy string   `json:"explicitContentPolicy"`
	Name                  string   `json:"name"`
	SupportedLanguageTags []string `json:"supportedLanguageTags"`
}

// Song is a catalog or library song resource.
type Song struct {
	ID            string                  `json:"id"`
	Type          ResourceType            `json:"type"`
	Href          string                  `json:"href,omitempty"`
	Attributes    SongAttributes          `json:"attributes"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

type SongAttributes struct {
	AlbumName            string         `json:"albumName"`
	ArtistName           string         `json:"artistName"`
	Artwork              Artwork        `json:"artwork"`
	ComposerName         string         `json:"composerName,omitempty"`
	ContentRating        string         `json:"contentRating,omitempty"`
	DiscNumber           int            `json:"discNumber"`
	DurationInMillis     int64          `json:"durationInMillis"`
	GenreNames           []string       `json:"genreNames"`
	HasLyrics            bool           `json:"hasLyrics"`
	IsAppleDigitalMaster bool           `json:"isAppleDigitalMaster"`
	ISRC                 string         `json:"isrc,omitempty"`
	Name                 string         `json:"name"`
	PlayParams           PlayParameters `json:"playParams"`
	Previews             []Preview      `json:"previews,omitempty"`
	ReleaseDate          string         `json:"releaseDate,omitempty"`
	TrackNumber          int            `json:"trackNumber"`
	URL                  string         `json:"url,omitempty"`
}

// Album is a catalog or library album resource.
type Album struct {
	ID            string                  `json:"id"`
	Type          ResourceType            `json:"type"`
	Href          string                  `json:"href,omitempty"`
	Attributes    AlbumAttributes         `json:"attributes"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

type AlbumAttributes struct {
	ArtistName     string         `json:"artistName"`
	Artwork        Artwork        `json:"artwork"`
	ContentRating  string         `json:"contentRating,omitempty"`
	Copyright      string         `json:"copyright,omitempty"`
	EditorialNotes EditorialNotes `json:"editorialNotes,omitempty"`
	GenreNames     []string       `json:"genreNames"`
	IsCompilation  bool           `json:"isCompilation"`
	IsComplete     bool           `json:"isComplete"`
	IsSingle       bool           `json:"isSingle"`
	Name           string         `json:"name"`
	PlayParams     PlayParameters `json:"playParams"`
	RecordLabel    string         `json:"recordLabel,omitempty"`
	ReleaseDate    string         `json:"releaseDate,omitempty"`
	TrackCount     int            `json:"trackCount"`
	UPC            string         `json:"upc,omitempty"`
	URL            string         `json:"url,omitempty"`
}

// Artist is a catalog or library artist resource.
type Artist struct {
	ID            string                  `json:"id"`
	Type          ResourceType            `json:"type"`
	Href          string                  `json:"href,omitempty"`
	Attributes    ArtistAttributes        `json:"attributes"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

type ArtistAttributes struct {
	Artwork        Artwork        `json:"artwork,omitempty"`
	EditorialNotes EditorialNotes `json:"editorialNotes,omitempty"`
	GenreNames     []string       `json:"genreNames,omitempty"`
	Name           string         `json:"name"`
	URL            string         `json:"url,omitempty"`
}

// Playlist is a catalog or library playlist resource.
type Playlist struct {
	ID            string                  `json:"id"`
	Type          ResourceType            `json:"type"`
	Href          string                  `json:"href,omitempty"`
	Attributes    PlaylistAttributes      `json:"attributes"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

type PlaylistAttributes struct {
	Artwork          Artwork              `json:"artwork,omitempty"`
	CanEdit          bool                 `json:"canEdit,omitempty"`
	CuratorName      string               `json:"curatorName,omitempty"`
	DateAdded        string               `json:"dateAdded,omitempty"`
	Description      DescriptionAttribute `json:"description,omitempty"`
	IsChart          bool                 `json:"isChart,omitempty"`
	IsPublic         bool                 `json:"isPublic,omitempty"`
	LastModifiedDate string               `json:"lastModifiedDate,omitempty"`
	Name             string               `json:"name"`
	PlaylistType     string               `json:"playlistType,omitempty"`
	PlayParams       PlayParameters       `json:"playParams,omitempty"`
	URL              string               `json:"url,omitempty"`
}

// MusicVideo is a catalog or library music-video resource.
type MusicVideo struct {
	ID            string                  `json:"id"`
	Type          ResourceType            `json:"type"`
	Href          string                  `json:"href,omitempty"`
	Attributes    MusicVideoAttributes    `json:"attributes"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

type MusicVideoAttributes struct {
	AlbumName        string         `json:"albumName,omitempty"`
	ArtistName       string         `json:"artistName"`
	Artwork          Artwork        `json:"artwork"`
	ContentRating    string         `json:"contentRating,omitempty"`
	DurationInMillis int64          `json:"durationInMillis"`
	GenreNames       []string       `json:"genreNames"`
	ISRC             string         `json:"isrc,omitempty"`
	Name             string         `json:"name"`
	PlayParams       PlayParameters `json:"playParams"`
	Previews         []Preview      `json:"previews,omitempty"`
	ReleaseDate      string         `json:"releaseDate,omitempty"`
	TrackNumber      int            `json:"trackNumber,omitempty"`
	URL              string         `json:"url,omitempty"`
}

// AnyResource preserves ordered heterogeneous Apple resources without
// pretending that all resource types share one attribute schema.
type AnyResource struct {
	ID            string                  `json:"id"`
	Type          ResourceType            `json:"type"`
	Href          string                  `json:"href,omitempty"`
	Attributes    json.RawMessage         `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

// Page contains typed resources and an opaque offset cursor.
type Page[T any] struct {
	Data       []T
	NextCursor *string
	Total      int
}

type apiCollection[T any] struct {
	Data []T    `json:"data"`
	Next string `json:"next"`
	Meta struct {
		Total int `json:"total"`
	} `json:"meta"`
}

// CatalogSearchResult contains one typed result collection per requested type.
type CatalogSearchResult struct {
	Songs       Page[Song]
	Albums      Page[Album]
	Artists     Page[Artist]
	Playlists   Page[Playlist]
	MusicVideos Page[MusicVideo]
}

// LibrarySearchResult contains typed personal-library search collections.
type LibrarySearchResult struct {
	Songs       Page[Song]
	Albums      Page[Album]
	Artists     Page[Artist]
	Playlists   Page[Playlist]
	MusicVideos Page[MusicVideo]
}

// Chart is one ranked catalog collection.
type Chart[T any] struct {
	Chart      string
	Name       string
	OrderID    string
	Data       []T
	NextCursor *string
}

// CatalogCharts contains typed chart groups.
type CatalogCharts struct {
	Songs       []Chart[Song]
	Albums      []Chart[Album]
	Playlists   []Chart[Playlist]
	MusicVideos []Chart[MusicVideo]
}

type apiChart[T any] struct {
	Chart   string `json:"chart"`
	Name    string `json:"name"`
	OrderID string `json:"orderId"`
	Next    string `json:"next"`
	Data    []T    `json:"data"`
}
