package trakt

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

type PageRequest struct {
	Cursor     string
	MaxResults int
	Extended   string
}

type SearchRequest struct {
	Query      string
	Types      []MediaType
	Fields     []string
	Cursor     string
	MaxResults int
	Extended   string
}

type HistoryRequest struct {
	Username   string
	Type       MediaType
	StartAt    time.Time
	EndAt      time.Time
	Cursor     string
	MaxResults int
	Extended   string
}

type WatchlistRequest struct {
	Username   string
	Type       MediaType
	Sort       string
	Cursor     string
	MaxResults int
	Extended   string
}

type RatingsRequest struct {
	Username   string
	Type       MediaType
	Rating     int
	Cursor     string
	MaxResults int
	Extended   string
}

type MovieRef struct {
	Title string `json:"title,omitempty"`
	Year  int    `json:"year,omitempty"`
	IDs   IDs    `json:"ids,omitzero"`
}

type ShowRef struct {
	Title string `json:"title,omitempty"`
	Year  int    `json:"year,omitempty"`
	IDs   IDs    `json:"ids,omitzero"`
}

type SeasonRef struct {
	Number int `json:"number,omitempty"`
	IDs    IDs `json:"ids"`
}

type EpisodeRef struct {
	IDs IDs `json:"ids"`
}

type HistoryMovie struct {
	MovieRef
	WatchedAt *time.Time `json:"watched_at,omitempty"`
}

type HistoryShow struct {
	ShowRef
	WatchedAt *time.Time `json:"watched_at,omitempty"`
}

type HistorySeason struct {
	SeasonRef
	WatchedAt *time.Time `json:"watched_at,omitempty"`
}

type HistoryEpisode struct {
	EpisodeRef
	WatchedAt *time.Time `json:"watched_at,omitempty"`
}

type HistoryMutation struct {
	Movies   []HistoryMovie   `json:"movies,omitempty"`
	Shows    []HistoryShow    `json:"shows,omitempty"`
	Seasons  []HistorySeason  `json:"seasons,omitempty"`
	Episodes []HistoryEpisode `json:"episodes,omitempty"`
	IDs      []int64          `json:"ids,omitempty"`
}

type MediaMutation struct {
	Movies   []MovieRef   `json:"movies,omitempty"`
	Shows    []ShowRef    `json:"shows,omitempty"`
	Seasons  []SeasonRef  `json:"seasons,omitempty"`
	Episodes []EpisodeRef `json:"episodes,omitempty"`
}

type RatedMovie struct {
	IDs    IDs `json:"ids"`
	Rating int `json:"rating,omitempty"`
}

type RatedShow struct {
	IDs    IDs `json:"ids"`
	Rating int `json:"rating,omitempty"`
}

type RatedSeason struct {
	IDs    IDs `json:"ids"`
	Rating int `json:"rating,omitempty"`
}

type RatedEpisode struct {
	IDs    IDs `json:"ids"`
	Rating int `json:"rating,omitempty"`
}

type RatingsMutation struct {
	Movies   []RatedMovie   `json:"movies,omitempty"`
	Shows    []RatedShow    `json:"shows,omitempty"`
	Seasons  []RatedSeason  `json:"seasons,omitempty"`
	Episodes []RatedEpisode `json:"episodes,omitempty"`
}

type MutationCounts struct {
	Movies   int `json:"movies"`
	Shows    int `json:"shows"`
	Seasons  int `json:"seasons"`
	Episodes int `json:"episodes"`
}

type SyncResult struct {
	Added    MutationCounts `json:"added"`
	Updated  MutationCounts `json:"updated"`
	Existing MutationCounts `json:"existing"`
	Deleted  MutationCounts `json:"deleted"`
	NotFound any            `json:"not_found,omitempty"`
	List     *struct {
		UpdatedAt time.Time `json:"updated_at"`
		ItemCount int       `json:"item_count"`
	} `json:"list,omitempty"`
}

type ScrobbleRequest struct {
	Progress float64     `json:"progress"`
	Movie    *MovieRef   `json:"movie,omitempty"`
	Episode  *EpisodeRef `json:"episode,omitempty"`
}

type ScrobbleResult struct {
	ID       int64    `json:"id"`
	Progress float64  `json:"progress"`
	Action   string   `json:"action"`
	Movie    *Movie   `json:"movie,omitempty"`
	Episode  *Episode `json:"episode,omitempty"`
	Show     *Show    `json:"show,omitempty"`
}

type CommentActivityRequest struct {
	Activity       string
	CommentType    string
	MediaType      string
	IncludeReplies bool
	Cursor         string
	MaxResults     int
}

type CommentTarget struct {
	Type    MediaType
	TraktID int64
}

type CreateCommentRequest struct {
	Target  CommentTarget
	Text    string
	Spoiler bool
}

type EditCommentRequest struct {
	ID      int64
	Text    string
	Spoiler bool
}

type OAuthWorkflow interface {
	AuthorizationURL(AuthorizationRequest) (string, error)
	Exchange(context.Context, string, string) (socialhub.Token, error)
	Refresh(context.Context, string, string) (socialhub.Token, error)
	RequestDeviceCode(context.Context) (*DeviceAuthorization, error)
	PollDevice(context.Context, DeviceAuthorization) (socialhub.Token, error)
	Revoke(context.Context, string) error
}

type CatalogWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[SearchResult], error)
	GetMovie(context.Context, string, string, ...socialhub.CallOption) (*Movie, error)
	GetShow(context.Context, string, string, ...socialhub.CallOption) (*Show, error)
	GetEpisode(context.Context, string, int, int, string, ...socialhub.CallOption) (*Episode, error)
	TrendingMovies(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[MovieTrend], error)
	PopularMovies(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[Movie], error)
	TrendingShows(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[ShowTrend], error)
	PopularShows(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[Show], error)
}

type UserWorkflow interface {
	GetProfile(context.Context, string, string, ...socialhub.CallOption) (*Profile, error)
	GetSettings(context.Context, ...socialhub.CallOption) (*UserSettings, error)
	ListHistory(context.Context, HistoryRequest, ...socialhub.CallOption) (socialhub.Page[HistoryItem], error)
	ListWatchlist(context.Context, WatchlistRequest, ...socialhub.CallOption) (socialhub.Page[WatchlistItem], error)
	ListRatings(context.Context, RatingsRequest, ...socialhub.CallOption) (socialhub.Page[RatingItem], error)
}

type SyncWorkflow interface {
	AddHistory(context.Context, HistoryMutation, ...socialhub.CallOption) (*SyncResult, error)
	RemoveHistory(context.Context, HistoryMutation, ...socialhub.CallOption) (*SyncResult, error)
	AddWatchlist(context.Context, MediaMutation, ...socialhub.CallOption) (*SyncResult, error)
	RemoveWatchlist(context.Context, MediaMutation, ...socialhub.CallOption) (*SyncResult, error)
	AddRatings(context.Context, RatingsMutation, ...socialhub.CallOption) (*SyncResult, error)
	RemoveRatings(context.Context, RatingsMutation, ...socialhub.CallOption) (*SyncResult, error)
}

type ScrobbleWorkflow interface {
	StartScrobble(context.Context, ScrobbleRequest, ...socialhub.CallOption) (*ScrobbleResult, error)
	PauseScrobble(context.Context, ScrobbleRequest, ...socialhub.CallOption) (*ScrobbleResult, error)
	StopScrobble(context.Context, ScrobbleRequest, ...socialhub.CallOption) (*ScrobbleResult, error)
}

type CommentWorkflow interface {
	ListComments(context.Context, CommentActivityRequest, ...socialhub.CallOption) (socialhub.Page[Comment], error)
	GetComment(context.Context, int64, ...socialhub.CallOption) (*Comment, error)
	ListReplies(context.Context, int64, PageRequest, ...socialhub.CallOption) (socialhub.Page[Comment], error)
	PostComment(context.Context, CreateCommentRequest, ...socialhub.CallOption) (*Comment, error)
	ReplyComment(context.Context, int64, string, bool, ...socialhub.CallOption) (*Comment, error)
	UpdateComment(context.Context, EditCommentRequest, ...socialhub.CallOption) (*Comment, error)
	DeleteComment(context.Context, int64, ...socialhub.CallOption) error
	LikeComment(context.Context, int64, ...socialhub.CallOption) error
	UnlikeComment(context.Context, int64, ...socialhub.CallOption) error
}

var _ OAuthWorkflow = (*Client)(nil)
var _ CatalogWorkflow = (*Client)(nil)
var _ UserWorkflow = (*Client)(nil)
var _ SyncWorkflow = (*Client)(nil)
var _ ScrobbleWorkflow = (*Client)(nil)
var _ CommentWorkflow = (*Client)(nil)
