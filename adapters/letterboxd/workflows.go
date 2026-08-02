package letterboxd

import (
	"context"

	"social-hub/pkg/socialhub"
)

type AuthorizationRequest struct {
	RedirectURI string
	State       string
	Scopes      []string
}

type PageRequest struct {
	Cursor  string
	PerPage int
}

type SearchRequest struct {
	Input        string
	Method       string
	IncludeTypes []string
	Cursor       string
	PerPage      int
}

type FilmListRequest struct {
	FilmIDs  []string
	Genre    string
	Country  string
	Language string
	Decade   int
	Year     int
	Sort     string
	Cursor   string
	PerPage  int
}

type LogEntriesRequest struct {
	FilmID    string
	MemberID  string
	Year      int
	Month     int
	MinRating float64
	MaxRating float64
	Where     []string
	Sort      string
	Cursor    string
	PerPage   int
}

type OAuthWorkflow interface {
	ClientCredentials(context.Context, []string) (socialhub.Token, error)
	AuthorizationURL(AuthorizationRequest) (string, error)
	Exchange(context.Context, string, string) (socialhub.Token, error)
	Refresh(context.Context, string) (socialhub.Token, error)
	Revoke(context.Context, string, string) error
}

type CatalogWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[SearchItem], error)
	GetFilm(context.Context, string, ...socialhub.CallOption) (*Film, error)
	ListFilms(context.Context, FilmListRequest, ...socialhub.CallOption) (socialhub.Page[FilmSummary], error)
}

type MemberWorkflow interface {
	GetMember(context.Context, string, ...socialhub.CallOption) (*Member, error)
	GetMe(context.Context, ...socialhub.CallOption) (*MemberAccount, error)
	ListActivity(context.Context, string, PageRequest, ...socialhub.CallOption) (socialhub.Page[ActivityItem], error)
	ListWatchlist(context.Context, string, FilmListRequest, ...socialhub.CallOption) (socialhub.Page[FilmSummary], error)
}

type LogEntryWorkflow interface {
	ListLogEntries(context.Context, LogEntriesRequest, ...socialhub.CallOption) (socialhub.Page[LogEntry], error)
	GetLogEntry(context.Context, string, ...socialhub.CallOption) (*LogEntry, error)
	ListReviewComments(context.Context, string, PageRequest, ...socialhub.CallOption) (socialhub.Page[ReviewComment], error)
	CreateLogEntry(context.Context, LogEntryCreationRequest, ...socialhub.CallOption) (*LogEntry, error)
	UpdateLogEntry(context.Context, string, LogEntryUpdateRequest, ...socialhub.CallOption) (*LogEntryUpdateResponse, error)
	DeleteLogEntry(context.Context, string, ...socialhub.CallOption) error
	CreateReviewComment(context.Context, string, string, ...socialhub.CallOption) (*ReviewComment, error)
}

type RelationshipWorkflow interface {
	SetLike(context.Context, string, bool, ...socialhub.CallOption) error
	SetRating(context.Context, string, *float64, ...socialhub.CallOption) error
	SetWatched(context.Context, string, bool, ...socialhub.CallOption) error
	SetWatchlist(context.Context, string, bool, ...socialhub.CallOption) error
}

var _ OAuthWorkflow = (*Client)(nil)
var _ CatalogWorkflow = (*Client)(nil)
var _ MemberWorkflow = (*Client)(nil)
var _ LogEntryWorkflow = (*Client)(nil)
var _ RelationshipWorkflow = (*Client)(nil)
