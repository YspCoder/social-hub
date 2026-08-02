package simkl

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

// AuthorizationRequest configures Simkl's confidential browser OAuth flow.
type AuthorizationRequest struct {
	RedirectURI string
	State       string
}

// PKCEAuthorizationRequest configures Simkl's public-client OAuth flow.
type PKCEAuthorizationRequest struct {
	RedirectURI string
	State       string
	PKCE        PKCE
}

// SearchRequest searches one Simkl catalog category.
type SearchRequest struct {
	Type     MediaType
	Query    string
	Cursor   string
	Limit    int
	Extended SearchExtended
}

// TrendingRequest selects a public CDN trending file.
type TrendingRequest struct {
	Type   MediaType
	Period TrendingPeriod
	Limit  int
}

// AllItemsRequest selects an incremental or initial library snapshot.
type AllItemsRequest struct {
	Type               SyncMediaType
	Status             WatchlistStatus
	DateFrom           *time.Time
	Extended           SyncExtended
	NextWatchInfo      bool
	EpisodeTVDBID      bool
	EpisodeWatchedAt   bool
	IncludeAllEpisodes IncludeEpisodes
	Memos              bool
}

// AddToListRequest batches watchlist placement under one target status.
type AddToListRequest struct {
	To     WatchlistStatus `json:"to"`
	Movies []MediaRef      `json:"movies,omitempty"`
	Shows  []MediaRef      `json:"shows,omitempty"`
	Anime  []MediaRef      `json:"anime,omitempty"`
}

// HistoryMutation batches movie, TV, and anime watch events.
type HistoryMutation struct {
	Movies []HistoryMedia  `json:"movies,omitempty"`
	Shows  []HistorySeries `json:"shows,omitempty"`
	Anime  []HistorySeries `json:"anime,omitempty"`
}

// RatingsMutation sets integer ratings from 1 through 10.
type RatingsMutation struct {
	Movies []MediaRating `json:"movies,omitempty"`
	Shows  []MediaRating `json:"shows,omitempty"`
	Anime  []MediaRating `json:"anime,omitempty"`
}

// RatingRemoval batches rating deletion by media identifier.
type RatingRemoval struct {
	Movies []MediaRef `json:"movies,omitempty"`
	Shows  []MediaRef `json:"shows,omitempty"`
	Anime  []MediaRef `json:"anime,omitempty"`
}

// ScrobbleRequest identifies one movie or one TV/anime episode.
type ScrobbleRequest struct {
	Progress float64             `json:"progress"`
	Movie    *MediaRef           `json:"movie,omitempty"`
	Show     *MediaRef           `json:"show,omitempty"`
	Anime    *MediaRef           `json:"anime,omitempty"`
	Episode  *PlaybackEpisodeRef `json:"episode,omitempty"`
}

type OAuthWorkflow interface {
	AuthorizationURL(AuthorizationRequest) (string, error)
	AuthorizationURLPKCE(PKCEAuthorizationRequest) (string, error)
	Exchange(context.Context, string, string) (socialhub.Token, error)
	ExchangePKCE(context.Context, string, string, string) (socialhub.Token, error)
	RequestPIN(context.Context, ...socialhub.CallOption) (*PINAuthorization, error)
	PollPIN(context.Context, PINAuthorization, ...socialhub.CallOption) (socialhub.Token, error)
}

type CatalogWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[SearchResult], error)
	GetMovie(context.Context, int64, ...socialhub.CallOption) (*MovieDetail, error)
	GetTV(context.Context, int64, ...socialhub.CallOption) (*TVDetail, error)
	GetAnime(context.Context, int64, ...socialhub.CallOption) (*AnimeDetail, error)
}

type TrendingWorkflow interface {
	ListTrending(context.Context, TrendingRequest, ...socialhub.CallOption) ([]TrendingItem, error)
}

type UserWorkflow interface {
	GetSettings(context.Context, ...socialhub.CallOption) (*UserSettings, error)
}

type SyncWorkflow interface {
	GetActivities(context.Context, ...socialhub.CallOption) (*Activities, error)
	ListAllItems(context.Context, AllItemsRequest, ...socialhub.CallOption) (*AllItems, error)
	AddToList(context.Context, AddToListRequest, ...socialhub.CallOption) (*ListMutationResult, error)
	AddHistory(context.Context, HistoryMutation, ...socialhub.CallOption) (*MutationResult, error)
	RemoveHistory(context.Context, HistoryMutation, ...socialhub.CallOption) (*MutationResult, error)
	AddRatings(context.Context, RatingsMutation, ...socialhub.CallOption) (*MutationResult, error)
	RemoveRatings(context.Context, RatingRemoval, ...socialhub.CallOption) (*MutationResult, error)
}

type ScrobbleWorkflow interface {
	Start(context.Context, ScrobbleRequest, ...socialhub.CallOption) (*ScrobbleResult, error)
	Pause(context.Context, ScrobbleRequest, ...socialhub.CallOption) (*ScrobbleResult, error)
	Stop(context.Context, ScrobbleRequest, ...socialhub.CallOption) (*ScrobbleResult, error)
	Checkin(context.Context, ScrobbleRequest, ...socialhub.CallOption) (*ScrobbleResult, error)
}

var _ OAuthWorkflow = (*Client)(nil)
var _ CatalogWorkflow = (*Client)(nil)
var _ TrendingWorkflow = (*Client)(nil)
var _ UserWorkflow = (*Client)(nil)
var _ SyncWorkflow = (*Client)(nil)
var _ ScrobbleWorkflow = (*Client)(nil)
