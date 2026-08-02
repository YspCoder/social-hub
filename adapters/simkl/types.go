package simkl

import (
	"encoding/json"
	"fmt"
	"time"
)

// MediaType identifies a Simkl catalog category.
type MediaType string

const (
	MediaMovie MediaType = "movie"
	MediaTV    MediaType = "tv"
	MediaAnime MediaType = "anime"
)

type SearchExtended string

const (
	SearchSimple SearchExtended = "simple"
	SearchFull   SearchExtended = "full"
)

type TrendingPeriod string

const (
	TrendingToday TrendingPeriod = "today"
	TrendingWeek  TrendingPeriod = "week"
	TrendingMonth TrendingPeriod = "month"
)

type SyncMediaType string

const (
	SyncMovies SyncMediaType = "movies"
	SyncShows  SyncMediaType = "shows"
	SyncAnime  SyncMediaType = "anime"
	SyncAll    SyncMediaType = "all"
)

type WatchlistStatus string

const (
	StatusWatching    WatchlistStatus = "watching"
	StatusPlanToWatch WatchlistStatus = "plantowatch"
	StatusHold        WatchlistStatus = "hold"
	StatusCompleted   WatchlistStatus = "completed"
	StatusDropped     WatchlistStatus = "dropped"
	StatusAll         WatchlistStatus = "all"
)

type SyncExtended string

const (
	SyncFull             SyncExtended = "full"
	SyncFullAnimeSeasons SyncExtended = "full_anime_seasons"
	SyncSimklIDsOnly     SyncExtended = "simkl_ids_only"
	SyncIDsOnly          SyncExtended = "ids_only"
)

type IncludeEpisodes string

const (
	IncludeEpisodesYes      IncludeEpisodes = "yes"
	IncludeEpisodesOriginal IncludeEpisodes = "original"
	IncludeEpisodesNo       IncludeEpisodes = "no"
)

// IDs contains Simkl and supported external media identifiers.
type IDs struct {
	Simkl       int64  `json:"simkl,omitempty"`
	Slug        string `json:"slug,omitempty"`
	IMDB        string `json:"imdb,omitempty"`
	TMDB        string `json:"tmdb,omitempty"`
	TVDB        string `json:"tvdb,omitempty"`
	MAL         string `json:"mal,omitempty"`
	AniDB       string `json:"anidb,omitempty"`
	AniList     string `json:"anilist,omitempty"`
	Kitsu       string `json:"kitsu,omitempty"`
	AniSearch   string `json:"anisearch,omitempty"`
	AnimePlanet string `json:"animeplanet,omitempty"`
	LiveChart   string `json:"livechart,omitempty"`
	Letterboxd  string `json:"letterboxd,omitempty"`
	Netflix     string `json:"netflix,omitempty"`
	Hulu        string `json:"hulu,omitempty"`
	Crunchyroll string `json:"crunchyroll,omitempty"`
	TraktSlug   string `json:"traktslug,omitempty"`
}

// UnmarshalJSON normalizes search/CDN ids.simkl_id and API ids.simkl.
func (ids *IDs) UnmarshalJSON(data []byte) error {
	type alias IDs
	var payload struct {
		alias
		SimklID int64 `json:"simkl_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	decoded := IDs(payload.alias)
	if decoded.Simkl != 0 && payload.SimklID != 0 && decoded.Simkl != payload.SimklID {
		return fmt.Errorf("simkl: conflicting ids.simkl and ids.simkl_id")
	}
	if decoded.Simkl == 0 {
		decoded.Simkl = payload.SimklID
	}
	*ids = decoded
	return nil
}

type Rating struct {
	Rating float64 `json:"rating,omitempty"`
	Votes  int64   `json:"votes,omitempty"`
	Rank   int     `json:"rank,omitempty"`
}

type Ratings struct {
	Simkl *Rating `json:"simkl,omitempty"`
	IMDB  *Rating `json:"imdb,omitempty"`
	MAL   *Rating `json:"mal,omitempty"`
}

type SearchResult struct {
	Title        string   `json:"title"`
	TitleEnglish string   `json:"title_en,omitempty"`
	TitleRomaji  string   `json:"title_romaji,omitempty"`
	AllTitles    []string `json:"all_titles,omitempty"`
	Year         int      `json:"year,omitempty"`
	EndpointType string   `json:"endpoint_type"`
	AnimeType    string   `json:"type,omitempty"`
	Poster       *string  `json:"poster,omitempty"`
	URL          string   `json:"url,omitempty"`
	EpisodeCount *int     `json:"ep_count,omitempty"`
	Rank         *int     `json:"rank,omitempty"`
	Status       string   `json:"status,omitempty"`
	Overview     string   `json:"overview,omitempty"`
	Genres       []string `json:"genres,omitempty"`
	Ratings      Ratings  `json:"ratings,omitempty"`
	IDs          IDs      `json:"ids"`
}

type AltTitle struct {
	Name string `json:"name"`
	Lang int    `json:"lang,omitempty"`
	Type string `json:"type,omitempty"`
}

type Trailer struct {
	Name    *string `json:"name,omitempty"`
	YouTube string  `json:"youtube"`
	Size    *int    `json:"size,omitempty"`
}

type BaseDetail struct {
	Title     string     `json:"title"`
	Year      int        `json:"year,omitempty"`
	Type      string     `json:"type"`
	IDs       IDs        `json:"ids"`
	Rank      *int       `json:"rank,omitempty"`
	DropRate  *string    `json:"droprate,omitempty"`
	Poster    *string    `json:"poster,omitempty"`
	Fanart    *string    `json:"fanart,omitempty"`
	Runtime   *int       `json:"runtime,omitempty"`
	Country   *string    `json:"country,omitempty"`
	Overview  *string    `json:"overview,omitempty"`
	Genres    []string   `json:"genres,omitempty"`
	AltTitles []AltTitle `json:"alt_titles,omitempty"`
	Ratings   Ratings    `json:"ratings,omitempty"`
	Trailers  []Trailer  `json:"trailers,omitempty"`
}

type MovieDetail struct {
	BaseDetail
	Released      *string `json:"released,omitempty"`
	Director      *string `json:"director,omitempty"`
	Certification *string `json:"certification,omitempty"`
	Budget        *int64  `json:"budget,omitempty"`
	Revenue       *int64  `json:"revenue,omitempty"`
	Language      string  `json:"language,omitempty"`
}

type TVDetail struct {
	BaseDetail
	Certification *string         `json:"certification,omitempty"`
	Network       *string         `json:"network,omitempty"`
	Status        *string         `json:"status,omitempty"`
	FirstAired    *string         `json:"first_aired,omitempty"`
	LastAired     *string         `json:"last_aired,omitempty"`
	Airs          json.RawMessage `json:"airs,omitempty"`
	TotalEpisodes *int            `json:"total_episodes,omitempty"`
	YearStartEnd  *string         `json:"year_start_end,omitempty"`
}

type AnimeRelation struct {
	Title        string `json:"title"`
	EnglishTitle string `json:"en_title,omitempty"`
	Year         int    `json:"year,omitempty"`
	AnimeType    string `json:"anime_type,omitempty"`
	RelationType string `json:"relation_type,omitempty"`
	Direct       bool   `json:"is_direct,omitempty"`
	IDs          IDs    `json:"ids"`
}

type AnimeDetail struct {
	BaseDetail
	EnglishTitle      *string         `json:"en_title,omitempty"`
	AnimeType         string          `json:"anime_type"`
	Certification     *string         `json:"certification,omitempty"`
	Network           *string         `json:"network,omitempty"`
	Status            *string         `json:"status,omitempty"`
	FirstAired        *string         `json:"first_aired,omitempty"`
	LastAired         *string         `json:"last_aired,omitempty"`
	Airs              json.RawMessage `json:"airs,omitempty"`
	TotalEpisodes     *int            `json:"total_episodes,omitempty"`
	YearStartEnd      *string         `json:"year_start_end,omitempty"`
	SeasonNameYear    *string         `json:"season_name_year,omitempty"`
	MappedTVDBSeasons []int           `json:"mapped_tvdb_seasons,omitempty"`
	Relations         []AnimeRelation `json:"relations,omitempty"`
}

type TrendingItem struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Poster        string   `json:"poster,omitempty"`
	Fanart        string   `json:"fanart,omitempty"`
	Rank          int      `json:"rank"`
	DropRate      string   `json:"drop_rate,omitempty"`
	Watched       int64    `json:"watched,omitempty"`
	PlanToWatch   int64    `json:"plan_to_watch,omitempty"`
	ReleaseDate   string   `json:"release_date,omitempty"`
	Country       string   `json:"country,omitempty"`
	Runtime       string   `json:"runtime,omitempty"`
	Status        string   `json:"status,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	Trailer       string   `json:"trailer,omitempty"`
	Overview      string   `json:"overview,omitempty"`
	Metadata      string   `json:"metadata,omitempty"`
	Ratings       Ratings  `json:"ratings,omitempty"`
	IDs           IDs      `json:"ids"`
	TotalEpisodes int      `json:"total_episodes,omitempty"`
	Network       string   `json:"network,omitempty"`
	AnimeType     string   `json:"anime_type,omitempty"`
	DVDDate       string   `json:"dvd_date,omitempty"`
	TheaterDate   string   `json:"theater,omitempty"`
}

type UserSettings struct {
	User struct {
		Name     string  `json:"name"`
		JoinedAt string  `json:"joined_at,omitempty"`
		Gender   string  `json:"gender,omitempty"`
		Avatar   string  `json:"avatar,omitempty"`
		Bio      string  `json:"bio,omitempty"`
		Location *string `json:"loc,omitempty"`
		Age      string  `json:"age,omitempty"`
	} `json:"user"`
	Account struct {
		ID       int64  `json:"id"`
		Timezone string `json:"timezone,omitempty"`
		Type     string `json:"type,omitempty"`
	} `json:"account"`
}

type ActivitySettings struct {
	All *time.Time `json:"all"`
}

type ActivitySeries struct {
	All             *time.Time `json:"all"`
	RatedAt         *time.Time `json:"rated_at"`
	Playback        *time.Time `json:"playback"`
	PlanToWatch     *time.Time `json:"plantowatch"`
	Watching        *time.Time `json:"watching"`
	Completed       *time.Time `json:"completed"`
	Hold            *time.Time `json:"hold"`
	Dropped         *time.Time `json:"dropped"`
	RemovedFromList *time.Time `json:"removed_from_list"`
}

type Activities struct {
	All      *time.Time       `json:"all"`
	Settings ActivitySettings `json:"settings"`
	TVShows  ActivitySeries   `json:"tv_shows"`
	Anime    ActivitySeries   `json:"anime"`
	Movies   ActivitySeries   `json:"movies"`
}

type MediaRef struct {
	Title string `json:"title,omitempty"`
	Year  int    `json:"year,omitempty"`
	IDs   IDs    `json:"ids"`
}

type Memo struct {
	Text      string `json:"text"`
	IsPrivate bool   `json:"is_private"`
}

type EpisodeRef struct {
	Number    int        `json:"number"`
	WatchedAt *time.Time `json:"watched_at,omitempty"`
	IDs       IDs        `json:"ids,omitzero"`
}

type SeasonRef struct {
	Number    int          `json:"number"`
	WatchedAt *time.Time   `json:"watched_at,omitempty"`
	Episodes  []EpisodeRef `json:"episodes,omitempty"`
}

type HistoryMedia struct {
	MediaRef
	WatchedAt *time.Time      `json:"watched_at,omitempty"`
	Status    WatchlistStatus `json:"status,omitempty"`
	Rating    int             `json:"rating,omitempty"`
	Memo      *Memo           `json:"memo,omitempty"`
}

type HistorySeries struct {
	HistoryMedia
	Seasons  []SeasonRef  `json:"seasons,omitempty"`
	Episodes []EpisodeRef `json:"episodes,omitempty"`
}

type MediaRating struct {
	MediaRef
	Rating  int        `json:"rating"`
	RatedAt *time.Time `json:"rated_at,omitempty"`
}

type ListMutationItem struct {
	Title string          `json:"title,omitempty"`
	Year  int             `json:"year,omitempty"`
	To    WatchlistStatus `json:"to,omitempty"`
	IDs   IDs             `json:"ids"`
}

type NotFoundReport struct {
	Movies   []MediaRef      `json:"movies,omitempty"`
	Shows    []HistorySeries `json:"shows,omitempty"`
	Episodes []HistorySeries `json:"episodes,omitempty"`
}

type ListMutationResult struct {
	Added struct {
		Movies []ListMutationItem `json:"movies,omitempty"`
		Shows  []ListMutationItem `json:"shows,omitempty"`
		Anime  []ListMutationItem `json:"anime,omitempty"`
	} `json:"added"`
	NotFound NotFoundReport `json:"not_found"`
}

type MutationCounts struct {
	Movies   int          `json:"movies"`
	Shows    int          `json:"shows"`
	Episodes int          `json:"episodes"`
	Statuses []SyncStatus `json:"statuses,omitempty"`
}

type SyncStatus struct {
	Request  json.RawMessage `json:"request,omitempty"`
	Response struct {
		Status    WatchlistStatus `json:"status,omitempty"`
		SimklType string          `json:"simkl_type,omitempty"`
		AnimeType *string         `json:"anime_type,omitempty"`
	} `json:"response,omitempty"`
}

type MutationResult struct {
	Added    MutationCounts `json:"added,omitempty"`
	Deleted  MutationCounts `json:"deleted,omitempty"`
	NotFound NotFoundReport `json:"not_found"`
}

type LibraryMedia struct {
	Title   string  `json:"title"`
	Poster  *string `json:"poster,omitempty"`
	Year    *int    `json:"year,omitempty"`
	Runtime *int    `json:"runtime,omitempty"`
	IDs     IDs     `json:"ids"`
}

type LibraryEpisode struct {
	Number    int        `json:"number"`
	WatchedAt *time.Time `json:"watched_at,omitempty"`
	TVDB      *struct {
		Season  int `json:"season"`
		Episode int `json:"episode"`
	} `json:"tvdb,omitempty"`
	IDs struct {
		TVDBID int64 `json:"tvdb_id,omitempty"`
	} `json:"ids,omitempty"`
}

type LibrarySeason struct {
	Number   int              `json:"number"`
	Episodes []LibraryEpisode `json:"episodes,omitempty"`
}

type NextWatchInfo struct {
	Title   string     `json:"title,omitempty"`
	Season  int        `json:"season,omitempty"`
	Episode int        `json:"episode,omitempty"`
	Date    *time.Time `json:"date,omitempty"`
}

type LibraryItem struct {
	AddedToWatchlistAt   *time.Time      `json:"added_to_watchlist_at,omitempty"`
	LastWatchedAt        *time.Time      `json:"last_watched_at,omitempty"`
	UserRatedAt          *time.Time      `json:"user_rated_at,omitempty"`
	UserRating           *int            `json:"user_rating,omitempty"`
	Status               WatchlistStatus `json:"status,omitempty"`
	LastWatched          *string         `json:"last_watched,omitempty"`
	NextToWatch          *string         `json:"next_to_watch,omitempty"`
	WatchedEpisodeCount  int             `json:"watched_episodes_count,omitempty"`
	TotalEpisodeCount    int             `json:"total_episodes_count,omitempty"`
	NotAiredEpisodeCount int             `json:"not_aired_episodes_count,omitempty"`
	Show                 *LibraryMedia   `json:"show,omitempty"`
	Movie                *LibraryMedia   `json:"movie,omitempty"`
	AnimeType            *string         `json:"anime_type,omitempty"`
	MappedTVDBSeasons    []int           `json:"mapped_tvdb_seasons,omitempty"`
	NextWatchInfo        *NextWatchInfo  `json:"next_to_watch_info,omitempty"`
	Memo                 json.RawMessage `json:"memo,omitempty"`
	Seasons              []LibrarySeason `json:"seasons,omitempty"`
}

type AllItems struct {
	Shows  []LibraryItem `json:"shows,omitempty"`
	Movies []LibraryItem `json:"movies,omitempty"`
	Anime  []LibraryItem `json:"anime,omitempty"`
}

type PlaybackEpisodeRef struct {
	Season int `json:"season,omitempty"`
	Number int `json:"number,omitempty"`
	IDs    IDs `json:"ids,omitzero"`
}

type ScrobbleResult struct {
	ID        int64     `json:"id,omitempty"`
	Action    string    `json:"action"`
	Progress  float64   `json:"progress"`
	SessionID string    `json:"sid,omitempty"`
	Movie     *MediaRef `json:"movie,omitempty"`
	Show      *MediaRef `json:"show,omitempty"`
	Anime     *MediaRef `json:"anime,omitempty"`
	Episode   *struct {
		Season int    `json:"season,omitempty"`
		Number int    `json:"number,omitempty"`
		Title  string `json:"title,omitempty"`
		IDs    IDs    `json:"ids,omitzero"`
	} `json:"episode,omitempty"`
	TVDBSeason int `json:"tvdb_season,omitempty"`
	TVDBNumber int `json:"tvdb_number,omitempty"`
}
