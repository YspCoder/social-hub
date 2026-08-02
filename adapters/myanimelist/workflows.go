package myanimelist

import (
	"context"

	"social-hub/pkg/socialhub"
)

type AuthorizationRequest struct {
	RedirectURI string
	State       string
	PKCE        PKCE
}

type PageRequest struct {
	Cursor string
	Limit  int
}

type SearchRequest struct {
	Query  string
	Cursor string
	Limit  int
}

type AnimeRankingRequest struct {
	Type   AnimeRankingType
	Cursor string
	Limit  int
}

type SeasonalAnimeRequest struct {
	Year   int
	Season AnimeSeason
	Sort   SeasonalAnimeSort
	Cursor string
	Limit  int
}

type MangaRankingRequest struct {
	Type   MangaRankingType
	Cursor string
	Limit  int
}

type AnimeListRequest struct {
	Username string
	Status   AnimeListState
	Sort     AnimeListSort
	Cursor   string
	Limit    int
}

type MangaListRequest struct {
	Username string
	Status   MangaListState
	Sort     MangaListSort
	Cursor   string
	Limit    int
}

type UpdateAnimeListStatusRequest struct {
	AnimeID            int64
	Status             *AnimeListState
	IsRewatching       *bool
	Score              *int
	NumWatchedEpisodes *int
	Priority           *int
	NumTimesRewatched  *int
	RewatchValue       *int
	Tags               []string
	Comments           *string
}

type UpdateMangaListStatusRequest struct {
	MangaID         int64
	Status          *MangaListState
	IsRereading     *bool
	Score           *int
	NumVolumesRead  *int
	NumChaptersRead *int
	Priority        *int
	NumTimesReread  *int
	RereadValue     *int
	Tags            []string
	Comments        *string
}

type OAuthWorkflow interface {
	AuthorizationURL(AuthorizationRequest) (string, error)
	Exchange(context.Context, string, string, string) (socialhub.Token, error)
	Refresh(context.Context, string) (socialhub.Token, error)
}

type AnimeWorkflow interface {
	SearchAnime(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Anime], error)
	GetAnime(context.Context, int64, ...socialhub.CallOption) (*Anime, error)
	ListAnimeRanking(context.Context, AnimeRankingRequest, ...socialhub.CallOption) (socialhub.Page[RankedAnime], error)
	ListSeasonalAnime(context.Context, SeasonalAnimeRequest, ...socialhub.CallOption) (socialhub.Page[Anime], error)
	ListAnimeSuggestions(context.Context, PageRequest, ...socialhub.CallOption) (socialhub.Page[Anime], error)
}

type MangaWorkflow interface {
	SearchManga(context.Context, SearchRequest, ...socialhub.CallOption) (socialhub.Page[Manga], error)
	GetManga(context.Context, int64, ...socialhub.CallOption) (*Manga, error)
	ListMangaRanking(context.Context, MangaRankingRequest, ...socialhub.CallOption) (socialhub.Page[RankedManga], error)
}

type UserWorkflow interface {
	GetMe(context.Context, ...socialhub.CallOption) (*User, error)
}

type AnimeListWorkflow interface {
	ListAnimeList(context.Context, AnimeListRequest, ...socialhub.CallOption) (socialhub.Page[AnimeListEntry], error)
	UpdateAnimeListStatus(context.Context, UpdateAnimeListStatusRequest, ...socialhub.CallOption) (*AnimeListStatus, error)
	DeleteAnimeListStatus(context.Context, int64, ...socialhub.CallOption) error
}

type MangaListWorkflow interface {
	ListMangaList(context.Context, MangaListRequest, ...socialhub.CallOption) (socialhub.Page[MangaListEntry], error)
	UpdateMangaListStatus(context.Context, UpdateMangaListStatusRequest, ...socialhub.CallOption) (*MangaListStatus, error)
	DeleteMangaListStatus(context.Context, int64, ...socialhub.CallOption) error
}

var _ OAuthWorkflow = (*Client)(nil)
var _ AnimeWorkflow = (*Client)(nil)
var _ MangaWorkflow = (*Client)(nil)
var _ UserWorkflow = (*Client)(nil)
var _ AnimeListWorkflow = (*Client)(nil)
var _ MangaListWorkflow = (*Client)(nil)
