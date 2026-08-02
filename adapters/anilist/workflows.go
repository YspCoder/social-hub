package anilist

import (
	"context"

	"social-hub/pkg/socialhub"
)

type AuthorizationRequest struct {
	RedirectURI string
	State       string
}

type ImplicitAuthorizationRequest struct {
	State string
}

type SearchMediaRequest struct {
	Query   string
	Type    MediaType
	Sort    MediaSort
	IsAdult *bool
	Cursor  string
	Limit   int
}

type ListMediaRequest struct {
	Type    MediaType
	Sort    MediaSort
	IsAdult *bool
	Cursor  string
	Limit   int
}

type SeasonalMediaRequest struct {
	Year    int
	Season  MediaSeason
	Sort    MediaSort
	IsAdult *bool
	Cursor  string
	Limit   int
}

type UserLookup struct {
	ID   int64
	Name string
}

type ListMediaListRequest struct {
	UserID   int64
	Username string
	Type     MediaType
	Status   MediaListStatus
	Sort     MediaListSort
	Cursor   string
	Limit    int
}

type SaveMediaListEntryRequest struct {
	ID                    int64
	MediaID               int64
	Status                *MediaListStatus
	Score                 *float64
	Progress              *int
	ProgressVolumes       *int
	Repeat                *int
	Priority              *int
	Private               *bool
	Notes                 *string
	HiddenFromStatusLists *bool
	CustomLists           []string
	StartedAt             *FuzzyDate
	CompletedAt           *FuzzyDate
}

type ListActivitiesRequest struct {
	UserID    int64
	MediaID   int64
	Types     []ActivityType
	Following bool
	Cursor    string
	Limit     int
}

type SaveTextActivityRequest struct {
	ID     int64
	Text   string
	Locked *bool
}

type OAuthWorkflow interface {
	AuthorizationURL(AuthorizationRequest) (string, error)
	ImplicitAuthorizationURL(ImplicitAuthorizationRequest) (string, error)
	Exchange(context.Context, string, string) (socialhub.Token, error)
}

type MediaWorkflow interface {
	SearchMedia(context.Context, SearchMediaRequest, ...socialhub.CallOption) (socialhub.Page[Media], error)
	GetMedia(context.Context, int64, ...socialhub.CallOption) (*Media, error)
	ListTrendingMedia(context.Context, ListMediaRequest, ...socialhub.CallOption) (socialhub.Page[Media], error)
	ListSeasonalMedia(context.Context, SeasonalMediaRequest, ...socialhub.CallOption) (socialhub.Page[Media], error)
}

type UserWorkflow interface {
	GetViewer(context.Context, ...socialhub.CallOption) (*User, error)
	GetUser(context.Context, UserLookup, ...socialhub.CallOption) (*User, error)
}

type MediaListWorkflow interface {
	ListMediaList(context.Context, ListMediaListRequest, ...socialhub.CallOption) (socialhub.Page[MediaListEntry], error)
	SaveMediaListEntry(context.Context, SaveMediaListEntryRequest, ...socialhub.CallOption) (*MediaListEntry, error)
	DeleteMediaListEntry(context.Context, int64, ...socialhub.CallOption) error
}

type ActivityWorkflow interface {
	ListActivities(context.Context, ListActivitiesRequest, ...socialhub.CallOption) (socialhub.Page[Activity], error)
	SaveTextActivity(context.Context, SaveTextActivityRequest, ...socialhub.CallOption) (*Activity, error)
	DeleteActivity(context.Context, int64, ...socialhub.CallOption) error
	ReplyActivity(context.Context, int64, string, ...socialhub.CallOption) (*ActivityReply, error)
	DeleteActivityReply(context.Context, int64, ...socialhub.CallOption) error
	ToggleLike(context.Context, int64, LikeableType, ...socialhub.CallOption) (*LikeResult, error)
}

var _ OAuthWorkflow = (*Client)(nil)
var _ MediaWorkflow = (*Client)(nil)
var _ UserWorkflow = (*Client)(nil)
var _ MediaListWorkflow = (*Client)(nil)
var _ ActivityWorkflow = (*Client)(nil)
