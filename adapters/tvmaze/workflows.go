package tvmaze

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

// CatalogWorkflow exposes TVmaze-specific television catalog reads.
type CatalogWorkflow interface {
	SearchShows(context.Context, string, ...socialhub.CallOption) ([]ShowSearchResult, error)
	GetShow(context.Context, int64, ...socialhub.CallOption) (*Show, error)
	LookupShow(context.Context, LookupShowRequest, ...socialhub.CallOption) (*Show, error)
	ListEpisodes(context.Context, int64, bool, ...socialhub.CallOption) ([]Episode, error)
	GetEpisode(context.Context, int64, ...socialhub.CallOption) (*Episode, error)
	GetEpisodeByNumber(context.Context, int64, int, int, ...socialhub.CallOption) (*Episode, error)
	ListEpisodesByDate(context.Context, int64, time.Time, ...socialhub.CallOption) ([]Episode, error)
	ListSeasons(context.Context, int64, ...socialhub.CallOption) ([]Season, error)
	ListSeasonEpisodes(context.Context, int64, ...socialhub.CallOption) ([]Episode, error)
	ListCast(context.Context, int64, ...socialhub.CallOption) ([]CastMember, error)
	ListCrew(context.Context, int64, ...socialhub.CallOption) ([]CrewMember, error)
}

// ScheduleWorkflow exposes broadcast and web-channel schedules.
type ScheduleWorkflow interface {
	ListSchedule(context.Context, ScheduleRequest, ...socialhub.CallOption) ([]Episode, error)
	ListWebSchedule(context.Context, WebScheduleRequest, ...socialhub.CallOption) ([]Episode, error)
}

// PeopleWorkflow exposes people search, details, and embedded show credits.
type PeopleWorkflow interface {
	SearchPeople(context.Context, string, ...socialhub.CallOption) ([]PersonSearchResult, error)
	GetPerson(context.Context, int64, ...socialhub.CallOption) (*Person, error)
	ListCastCredits(context.Context, int64, ...socialhub.CallOption) ([]CastCredit, error)
	ListCrewCredits(context.Context, int64, ...socialhub.CallOption) ([]CrewCredit, error)
}

// UpdatesWorkflow exposes timestamps for incremental catalog synchronization.
type UpdatesWorkflow interface {
	ListShowUpdates(context.Context, UpdatePeriod, ...socialhub.CallOption) ([]Update, error)
	ListPeopleUpdates(context.Context, UpdatePeriod, ...socialhub.CallOption) ([]Update, error)
}

var (
	_ CatalogWorkflow  = (*Client)(nil)
	_ ScheduleWorkflow = (*Client)(nil)
	_ PeopleWorkflow   = (*Client)(nil)
	_ UpdatesWorkflow  = (*Client)(nil)
)
