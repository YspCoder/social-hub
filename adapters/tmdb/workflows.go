package tmdb

import (
	"context"

	"social-hub/pkg/socialhub"
)

type PageRequest struct {
	Cursor   string
	Language string
}

type SearchRequest struct {
	Query        string
	IncludeAdult bool
	Language     string
	Cursor       string
}

type TrendingRequest struct {
	MediaType MediaType
	Window    string
	Language  string
	Cursor    string
}

type LibraryKind string

const (
	LibraryFavorites LibraryKind = "favorite"
	LibraryWatchlist LibraryKind = "watchlist"
	LibraryRated     LibraryKind = "rated"
)

type LibraryRequest struct {
	Kind      LibraryKind
	MediaType MediaType
	Language  string
	Sort      string
	Cursor    string
}

type MediaTarget struct {
	MediaType MediaType
	MediaID   int64
}

type RatingRequest struct {
	Target MediaTarget
	Value  float64
}

type AuthWorkflow interface {
	RequestToken(context.Context) (*RequestToken, error)
	ApprovalURL(string, string) (string, error)
	CreateSession(context.Context, string) (string, error)
	DeleteSession(context.Context, string) error
	CreateGuestSession(context.Context) (*GuestSession, error)
}

type CatalogWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[MediaItem], error)
	GetMovie(context.Context, int64, string, ...socialhub.CallOption) (*Movie, error)
	GetTVSeries(context.Context, int64, string, ...socialhub.CallOption) (*TVSeries, error)
	GetPerson(context.Context, int64, string, ...socialhub.CallOption) (*Person, error)
	Trending(context.Context, TrendingRequest, ...socialhub.CallOption) (socialhub.Page[MediaItem], error)
	PopularMovies(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[MediaItem], error)
	PopularTV(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[MediaItem], error)
	GetConfiguration(context.Context, ...socialhub.CallOption) (*Configuration, error)
}

type AccountWorkflow interface {
	GetAccount(context.Context, ...socialhub.CallOption) (*Account, error)
}

type LibraryWorkflow interface {
	ListLibrary(context.Context, LibraryRequest, ...socialhub.CallOption) (socialhub.Page[MediaItem], error)
	SetFavorite(context.Context, MediaTarget, bool, ...socialhub.CallOption) (*StatusResponse, error)
	SetWatchlist(context.Context, MediaTarget, bool, ...socialhub.CallOption) (*StatusResponse, error)
	SetRating(context.Context, RatingRequest, ...socialhub.CallOption) (*StatusResponse, error)
	DeleteRating(context.Context, MediaTarget, ...socialhub.CallOption) (*StatusResponse, error)
}

var _ AuthWorkflow = (*Client)(nil)
var _ CatalogWorkflow = (*Client)(nil)
var _ AccountWorkflow = (*Client)(nil)
var _ LibraryWorkflow = (*Client)(nil)
