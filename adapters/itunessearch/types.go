package itunessearch

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	DefaultSearchLimit = 50
	MaximumLimit       = 200
)

type Media string

const (
	MediaMovie      Media = "movie"
	MediaPodcast    Media = "podcast"
	MediaMusic      Media = "music"
	MediaMusicVideo Media = "musicVideo"
	MediaAudiobook  Media = "audiobook"
	MediaShortFilm  Media = "shortFilm"
	MediaTVShow     Media = "tvShow"
	MediaSoftware   Media = "software"
	MediaEBook      Media = "ebook"
	MediaAll        Media = "all"
)

type Entity string

const (
	EntityMovieArtist     Entity = "movieArtist"
	EntityMovie           Entity = "movie"
	EntityPodcastAuthor   Entity = "podcastAuthor"
	EntityPodcast         Entity = "podcast"
	EntityPodcastEpisode  Entity = "podcastEpisode"
	EntityMusicArtist     Entity = "musicArtist"
	EntityMusicTrack      Entity = "musicTrack"
	EntityAlbum           Entity = "album"
	EntityMusicVideo      Entity = "musicVideo"
	EntityMix             Entity = "mix"
	EntitySong            Entity = "song"
	EntityAudiobookAuthor Entity = "audiobookAuthor"
	EntityAudiobook       Entity = "audiobook"
	EntityShortFilmArtist Entity = "shortFilmArtist"
	EntityShortFilm       Entity = "shortFilm"
	EntityTVEpisode       Entity = "tvEpisode"
	EntityTVSeason        Entity = "tvSeason"
	EntitySoftware        Entity = "software"
	EntityIPadSoftware    Entity = "iPadSoftware"
	EntityDesktopSoftware Entity = "desktopSoftware"
	EntityEBook           Entity = "ebook"
	EntityAllArtist       Entity = "allArtist"
	EntityAllTrack        Entity = "allTrack"
)

type Attribute string

const (
	AttributeActorTerm         Attribute = "actorTerm"
	AttributeGenreIndex        Attribute = "genreIndex"
	AttributeArtistTerm        Attribute = "artistTerm"
	AttributeShortFilmTerm     Attribute = "shortFilmTerm"
	AttributeProducerTerm      Attribute = "producerTerm"
	AttributeRatingTerm        Attribute = "ratingTerm"
	AttributeDirectorTerm      Attribute = "directorTerm"
	AttributeReleaseYearTerm   Attribute = "releaseYearTerm"
	AttributeFeatureFilmTerm   Attribute = "featureFilmTerm"
	AttributeMovieArtistTerm   Attribute = "movieArtistTerm"
	AttributeMovieTerm         Attribute = "movieTerm"
	AttributeRatingIndex       Attribute = "ratingIndex"
	AttributeDescriptionTerm   Attribute = "descriptionTerm"
	AttributeTitleTerm         Attribute = "titleTerm"
	AttributeLanguageTerm      Attribute = "languageTerm"
	AttributeAuthorTerm        Attribute = "authorTerm"
	AttributeKeywordsTerm      Attribute = "keywordsTerm"
	AttributeMixTerm           Attribute = "mixTerm"
	AttributeComposerTerm      Attribute = "composerTerm"
	AttributeAlbumTerm         Attribute = "albumTerm"
	AttributeSongTerm          Attribute = "songTerm"
	AttributeSoftwareDeveloper Attribute = "softwareDeveloper"
	AttributeTVEpisodeTerm     Attribute = "tvEpisodeTerm"
	AttributeShowTerm          Attribute = "showTerm"
	AttributeTVSeasonTerm      Attribute = "tvSeasonTerm"
	AttributeAllArtistTerm     Attribute = "allArtistTerm"
	AttributeAllTrackTerm      Attribute = "allTrackTerm"
)

type Language string

const (
	LanguageEnglishUS Language = "en_us"
	LanguageJapanese  Language = "ja_jp"
)

type ResultVersion int

const (
	ResultVersionOne ResultVersion = 1
	ResultVersionTwo ResultVersion = 2
)

type ExplicitSetting string

const (
	ExplicitInclude ExplicitSetting = "Yes"
	ExplicitExclude ExplicitSetting = "No"
)

type LookupSort string

const LookupSortRecent LookupSort = "recent"

// SearchRequest describes one deterministic Store catalog search. Empty
// Country, Media, Limit, Language, Version, and Explicit fields use Apple's
// documented defaults and are normalized before the request is sent.
type SearchRequest struct {
	Term      string
	Country   string
	Media     Media
	Entity    Entity
	Attribute Attribute
	Limit     int
	Language  Language
	Version   ResultVersion
	Explicit  ExplicitSetting
}

// LookupRequest must select exactly one identifier family. Numeric families
// accept multiple positive IDs; UPC/EAN and ISBN lookups accept one value.
type LookupRequest struct {
	IDs          []int64
	AMGArtistIDs []int64
	AMGAlbumIDs  []int64
	AMGVideoIDs  []int64
	UPC          string
	ISBN         string
	Entity       Entity
	Limit        int
	Sort         LookupSort
}

// Result is the public cross-media result shape. URL fields are untrusted
// external metadata and are never followed or downloaded by this package.
type Result struct {
	WrapperType string `json:"wrapperType"`
	Kind        string `json:"kind"`

	ArtistID           *int64  `json:"artistId"`
	ArtistIDs          []int64 `json:"artistIds"`
	AMGArtistID        *int64  `json:"amgArtistId"`
	CollectionArtistID *int64  `json:"collectionArtistId"`
	CollectionID       *int64  `json:"collectionId"`
	TrackID            *int64  `json:"trackId"`
	PrimaryGenreID     *int64  `json:"primaryGenreId"`

	ArtistName             string `json:"artistName"`
	ArtistType             string `json:"artistType"`
	CollectionArtistName   string `json:"collectionArtistName"`
	CollectionName         string `json:"collectionName"`
	CollectionCensoredName string `json:"collectionCensoredName"`
	TrackName              string `json:"trackName"`
	TrackCensoredName      string `json:"trackCensoredName"`

	ArtistViewURL     string `json:"artistViewUrl"`
	ArtistLinkURL     string `json:"artistLinkUrl"`
	CollectionViewURL string `json:"collectionViewUrl"`
	TrackViewURL      string `json:"trackViewUrl"`
	PreviewURL        string `json:"previewUrl"`
	FeedURL           string `json:"feedUrl"`
	EpisodeURL        string `json:"episodeUrl"`
	SellerURL         string `json:"sellerUrl"`

	ArtworkURL30  string `json:"artworkUrl30"`
	ArtworkURL60  string `json:"artworkUrl60"`
	ArtworkURL100 string `json:"artworkUrl100"`
	ArtworkURL160 string `json:"artworkUrl160"`
	ArtworkURL512 string `json:"artworkUrl512"`
	ArtworkURL600 string `json:"artworkUrl600"`

	CollectionPrice        *float64 `json:"collectionPrice"`
	CollectionHDPrice      *float64 `json:"collectionHdPrice"`
	TrackPrice             *float64 `json:"trackPrice"`
	TrackHDPrice           *float64 `json:"trackHdPrice"`
	TrackRentalPrice       *float64 `json:"trackRentalPrice"`
	TrackHDRentalPrice     *float64 `json:"trackHdRentalPrice"`
	Price                  *float64 `json:"price"`
	FormattedPrice         string   `json:"formattedPrice"`
	CollectionExplicitness string   `json:"collectionExplicitness"`
	TrackExplicitness      string   `json:"trackExplicitness"`

	DiscCount       *int64 `json:"discCount"`
	DiscNumber      *int64 `json:"discNumber"`
	TrackCount      *int64 `json:"trackCount"`
	TrackNumber     *int64 `json:"trackNumber"`
	TrackTimeMillis *int64 `json:"trackTimeMillis"`

	Country          string   `json:"country"`
	Currency         string   `json:"currency"`
	PrimaryGenreName string   `json:"primaryGenreName"`
	GenreIDs         []string `json:"genreIds"`
	Genres           []string `json:"genres"`
	ReleaseDate      string   `json:"releaseDate"`
	Copyright        string   `json:"copyright"`

	Description           string `json:"description"`
	ShortDescription      string `json:"shortDescription"`
	LongDescription       string `json:"longDescription"`
	ContentAdvisoryRating string `json:"contentAdvisoryRating"`
	TrackContentRating    string `json:"trackContentRating"`

	BundleID                  string   `json:"bundleId"`
	Version                   string   `json:"version"`
	CurrentVersionReleaseDate string   `json:"currentVersionReleaseDate"`
	ReleaseNotes              string   `json:"releaseNotes"`
	MinimumOSVersion          string   `json:"minimumOsVersion"`
	FileSizeBytes             string   `json:"fileSizeBytes"`
	SellerName                string   `json:"sellerName"`
	LanguageCodesISO2A        []string `json:"languageCodesISO2A"`
	SupportedDevices          []string `json:"supportedDevices"`
	Features                  []string `json:"features"`
	Advisories                []string `json:"advisories"`
	ScreenshotURLs            []string `json:"screenshotUrls"`
	IPadScreenshotURLs        []string `json:"ipadScreenshotUrls"`
	AppleTVScreenshotURLs     []string `json:"appletvScreenshotUrls"`

	AverageUserRating                  *float64 `json:"averageUserRating"`
	AverageUserRatingForCurrentVersion *float64 `json:"averageUserRatingForCurrentVersion"`
	UserRatingCount                    *int64   `json:"userRatingCount"`
	UserRatingCountForCurrentVersion   *int64   `json:"userRatingCountForCurrentVersion"`
	IsStreamable                       *bool    `json:"isStreamable"`
	IsGameCenterEnabled                *bool    `json:"isGameCenterEnabled"`
	IsVPPDeviceBasedLicensingEnabled   *bool    `json:"isVppDeviceBasedLicensingEnabled"`

	EpisodeGUID          string `json:"episodeGuid"`
	EpisodeContentType   string `json:"episodeContentType"`
	EpisodeFileExtension string `json:"episodeFileExtension"`
	ClosedCaptioning     string `json:"closedCaptioning"`
}

type ResponseMeta struct {
	RequestID  string
	RetryAfter time.Duration
}

// CatalogResponse preserves Apple's resultCount/results envelope.
type CatalogResponse struct {
	ResultCount int          `json:"resultCount"`
	Results     []Result     `json:"results"`
	Meta        ResponseMeta `json:"-"`
}

type CatalogWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (*CatalogResponse, error)
	Lookup(context.Context, LookupRequest, ...socialhub.CallOption) (*CatalogResponse, error)
}

var _ CatalogWorkflow = (*Client)(nil)
