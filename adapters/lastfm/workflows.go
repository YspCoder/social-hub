package lastfm

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

// PageRequest selects a Last.fm page. Cursor is a positive decimal page number.
type PageRequest struct {
	Cursor     string
	MaxResults int
}

// SearchRequest selects a relevance-sorted search page.
type SearchRequest struct {
	Query      string
	Artist     string
	Cursor     string
	MaxResults int
}

// ArtistInfoRequest identifies an artist by MBID or name.
type ArtistInfoRequest struct {
	Artist      string
	MBID        string
	Language    string
	Username    string
	Autocorrect bool
}

// AlbumInfoRequest identifies an album by MBID or artist and album names.
type AlbumInfoRequest struct {
	Artist      string
	Album       string
	MBID        string
	Language    string
	Username    string
	Autocorrect bool
}

// TrackInfoRequest identifies a track by MBID or artist and track names.
type TrackInfoRequest struct {
	Artist      string
	Track       string
	MBID        string
	Username    string
	Autocorrect bool
}

// RecentTracksRequest selects listening history for one user.
type RecentTracksRequest struct {
	Username   string
	From       time.Time
	To         time.Time
	Extended   bool
	Cursor     string
	MaxResults int
}

// TopTracksPeriod is one of Last.fm's fixed aggregation periods.
type TopTracksPeriod string

const (
	PeriodOverall TopTracksPeriod = "overall"
	Period7Day    TopTracksPeriod = "7day"
	Period1Month  TopTracksPeriod = "1month"
	Period3Month  TopTracksPeriod = "3month"
	Period6Month  TopTracksPeriod = "6month"
	Period12Month TopTracksPeriod = "12month"
)

// TopTracksRequest selects one user's most-played tracks.
type TopTracksRequest struct {
	Username   string
	Period     TopTracksPeriod
	Cursor     string
	MaxResults int
}

// UserTracksRequest selects one page of a user's loved tracks.
type UserTracksRequest struct {
	Username   string
	Cursor     string
	MaxResults int
}

// TrackRef identifies a track for write operations.
type TrackRef struct {
	Artist string
	Track  string
}

// NowPlayingRequest describes the track currently playing.
type NowPlayingRequest struct {
	Artist      string
	Track       string
	Album       string
	AlbumArtist string
	TrackNumber int
	MBID        string
	Duration    time.Duration
}

// Scrobble records one completed or sufficiently played track.
type Scrobble struct {
	Artist       string
	Track        string
	StartedAt    time.Time
	Album        string
	AlbumArtist  string
	TrackNumber  int
	MBID         string
	Duration     time.Duration
	ChosenByUser *bool
}

// Correction records a value and whether Last.fm corrected it.
type Correction struct {
	Value     string `json:"value"`
	Corrected bool   `json:"corrected"`
}

// NowPlayingResult is Last.fm's normalized now-playing acknowledgement.
type NowPlayingResult struct {
	Track       Correction `json:"track"`
	Artist      Correction `json:"artist"`
	Album       Correction `json:"album"`
	AlbumArtist Correction `json:"album_artist"`
}

// ScrobbleItemResult is one accepted or ignored batch item.
type ScrobbleItemResult struct {
	Track          Correction `json:"track"`
	Artist         Correction `json:"artist"`
	Album          Correction `json:"album"`
	AlbumArtist    Correction `json:"album_artist"`
	Timestamp      time.Time  `json:"timestamp"`
	IgnoredCode    int        `json:"ignored_code"`
	IgnoredMessage string     `json:"ignored_message,omitempty"`
}

// ScrobbleResult summarizes a batch submission.
type ScrobbleResult struct {
	Accepted int                  `json:"accepted"`
	Ignored  int                  `json:"ignored"`
	Items    []ScrobbleItemResult `json:"items"`
}

type AuthWorkflow interface {
	RequestToken(context.Context, ...socialhub.CallOption) (string, error)
	AuthorizationURL(string, string) (string, error)
	ExchangeSession(context.Context, string, ...socialhub.CallOption) (*Session, error)
}

type DiscoveryWorkflow interface {
	GetTrack(context.Context, TrackInfoRequest, ...socialhub.CallOption) (*Track, error)
	SearchTracks(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Track], error)
	GetArtist(context.Context, ArtistInfoRequest, ...socialhub.CallOption) (*Artist, error)
	SearchArtists(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Artist], error)
	GetAlbum(context.Context, AlbumInfoRequest, ...socialhub.CallOption) (*Album, error)
	SearchAlbums(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Album], error)
}

type UserWorkflow interface {
	GetUser(context.Context, string, ...socialhub.CallOption) (*User, error)
	RecentTracks(context.Context, RecentTracksRequest, ...socialhub.CallOption) (socialhub.Page[Track], error)
	TopTracks(context.Context, TopTracksRequest, ...socialhub.CallOption) (socialhub.Page[Track], error)
	LovedTracks(context.Context, UserTracksRequest, ...socialhub.CallOption) (socialhub.Page[Track], error)
}

type ListeningWorkflow interface {
	UpdateNowPlaying(context.Context, NowPlayingRequest, ...socialhub.CallOption) (*NowPlayingResult, error)
	Scrobble(context.Context, []Scrobble, ...socialhub.CallOption) (*ScrobbleResult, error)
}

type LibraryWorkflow interface {
	LoveTrack(context.Context, TrackRef, ...socialhub.CallOption) error
	UnloveTrack(context.Context, TrackRef, ...socialhub.CallOption) error
}

var _ AuthWorkflow = (*Client)(nil)
var _ DiscoveryWorkflow = (*Client)(nil)
var _ UserWorkflow = (*Client)(nil)
var _ ListeningWorkflow = (*Client)(nil)
var _ LibraryWorkflow = (*Client)(nil)
