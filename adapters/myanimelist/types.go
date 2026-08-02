package myanimelist

import "time"

type AnimeRankingType string

const (
	AnimeRankingAll        AnimeRankingType = "all"
	AnimeRankingAiring     AnimeRankingType = "airing"
	AnimeRankingUpcoming   AnimeRankingType = "upcoming"
	AnimeRankingTV         AnimeRankingType = "tv"
	AnimeRankingOVA        AnimeRankingType = "ova"
	AnimeRankingMovie      AnimeRankingType = "movie"
	AnimeRankingSpecial    AnimeRankingType = "special"
	AnimeRankingPopularity AnimeRankingType = "bypopularity"
	AnimeRankingFavorite   AnimeRankingType = "favorite"
)

type MangaRankingType string

const (
	MangaRankingAll        MangaRankingType = "all"
	MangaRankingManga      MangaRankingType = "manga"
	MangaRankingNovels     MangaRankingType = "novels"
	MangaRankingOneShots   MangaRankingType = "oneshots"
	MangaRankingDoujin     MangaRankingType = "doujin"
	MangaRankingManhwa     MangaRankingType = "manhwa"
	MangaRankingManhua     MangaRankingType = "manhua"
	MangaRankingPopularity MangaRankingType = "bypopularity"
	MangaRankingFavorite   MangaRankingType = "favorite"
)

type AnimeSeason string

const (
	SeasonWinter AnimeSeason = "winter"
	SeasonSpring AnimeSeason = "spring"
	SeasonSummer AnimeSeason = "summer"
	SeasonFall   AnimeSeason = "fall"
)

type SeasonalAnimeSort string

const (
	SeasonalSortScore     SeasonalAnimeSort = "anime_score"
	SeasonalSortListUsers SeasonalAnimeSort = "anime_num_list_users"
)

type AnimeListState string

const (
	AnimeWatching    AnimeListState = "watching"
	AnimeCompleted   AnimeListState = "completed"
	AnimeOnHold      AnimeListState = "on_hold"
	AnimeDropped     AnimeListState = "dropped"
	AnimePlanToWatch AnimeListState = "plan_to_watch"
)

type MangaListState string

const (
	MangaReading    MangaListState = "reading"
	MangaCompleted  MangaListState = "completed"
	MangaOnHold     MangaListState = "on_hold"
	MangaDropped    MangaListState = "dropped"
	MangaPlanToRead MangaListState = "plan_to_read"
)

type AnimeListSort string

const (
	AnimeListSortScore     AnimeListSort = "list_score"
	AnimeListSortUpdatedAt AnimeListSort = "list_updated_at"
	AnimeListSortTitle     AnimeListSort = "anime_title"
	AnimeListSortStartDate AnimeListSort = "anime_start_date"
)

type MangaListSort string

const (
	MangaListSortScore     MangaListSort = "list_score"
	MangaListSortUpdatedAt MangaListSort = "list_updated_at"
	MangaListSortTitle     MangaListSort = "manga_title"
	MangaListSortStartDate MangaListSort = "manga_start_date"
)

type Picture struct {
	Medium string `json:"medium"`
	Large  string `json:"large,omitempty"`
}

type AlternativeTitles struct {
	Synonyms []string `json:"synonyms,omitempty"`
	English  string   `json:"en,omitempty"`
	Japanese string   `json:"ja,omitempty"`
}

type NamedResource struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Person struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type PersonRole struct {
	Person Person `json:"node"`
	Role   string `json:"role"`
}

type MagazineRole struct {
	Magazine NamedResource `json:"node"`
	Role     string        `json:"role"`
}

// Work contains fields shared by anime and manga catalog resources.
type Work struct {
	ID                int64             `json:"id"`
	Title             string            `json:"title"`
	MainPicture       *Picture          `json:"main_picture,omitempty"`
	AlternativeTitles AlternativeTitles `json:"alternative_titles,omitempty"`
	StartDate         string            `json:"start_date,omitempty"`
	EndDate           string            `json:"end_date,omitempty"`
	Synopsis          string            `json:"synopsis,omitempty"`
	Mean              float64           `json:"mean,omitempty"`
	Rank              int               `json:"rank,omitempty"`
	Popularity        int               `json:"popularity,omitempty"`
	NumListUsers      int               `json:"num_list_users,omitempty"`
	NumScoringUsers   int               `json:"num_scoring_users,omitempty"`
	NSFW              string            `json:"nsfw,omitempty"`
	Genres            []NamedResource   `json:"genres,omitempty"`
	CreatedAt         time.Time         `json:"created_at,omitempty"`
	UpdatedAt         time.Time         `json:"updated_at,omitempty"`
}

type StartSeason struct {
	Year   int         `json:"year"`
	Season AnimeSeason `json:"season"`
}

type Broadcast struct {
	DayOfWeek string `json:"day_of_the_week"`
	StartTime string `json:"start_time,omitempty"`
}

type AnimeStatistics struct {
	NumListUsers int `json:"num_list_users"`
	Status       struct {
		Watching    int `json:"watching"`
		Completed   int `json:"completed"`
		OnHold      int `json:"on_hold"`
		Dropped     int `json:"dropped"`
		PlanToWatch int `json:"plan_to_watch"`
	} `json:"status"`
}

type AnimeRelation struct {
	Anime                 Anime  `json:"node"`
	RelationType          string `json:"relation_type"`
	RelationTypeFormatted string `json:"relation_type_formatted"`
}

type MangaRelation struct {
	Manga                 Manga  `json:"node"`
	RelationType          string `json:"relation_type"`
	RelationTypeFormatted string `json:"relation_type_formatted"`
}

type AnimeRecommendation struct {
	Anime              Anime `json:"node"`
	NumRecommendations int   `json:"num_recommendations"`
}

type MangaRecommendation struct {
	Manga              Manga `json:"node"`
	NumRecommendations int   `json:"num_recommendations"`
}

// Anime contains the official API v2 catalog and optional user-list fields.
type Anime struct {
	Work
	MediaType              string                `json:"media_type,omitempty"`
	Status                 string                `json:"status,omitempty"`
	MyListStatus           *AnimeListStatus      `json:"my_list_status,omitempty"`
	NumEpisodes            int                   `json:"num_episodes,omitempty"`
	StartSeason            *StartSeason          `json:"start_season,omitempty"`
	Broadcast              *Broadcast            `json:"broadcast,omitempty"`
	Source                 string                `json:"source,omitempty"`
	AverageEpisodeDuration int                   `json:"average_episode_duration,omitempty"`
	Rating                 string                `json:"rating,omitempty"`
	Studios                []NamedResource       `json:"studios,omitempty"`
	Pictures               []Picture             `json:"pictures,omitempty"`
	Background             string                `json:"background,omitempty"`
	RelatedAnime           []AnimeRelation       `json:"related_anime,omitempty"`
	RelatedManga           []MangaRelation       `json:"related_manga,omitempty"`
	Recommendations        []AnimeRecommendation `json:"recommendations,omitempty"`
	Statistics             *AnimeStatistics      `json:"statistics,omitempty"`
}

// Manga contains the official API v2 catalog and optional user-list fields.
type Manga struct {
	Work
	MediaType       string                `json:"media_type,omitempty"`
	Status          string                `json:"status,omitempty"`
	MyListStatus    *MangaListStatus      `json:"my_list_status,omitempty"`
	NumVolumes      int                   `json:"num_volumes,omitempty"`
	NumChapters     int                   `json:"num_chapters,omitempty"`
	Authors         []PersonRole          `json:"authors,omitempty"`
	Pictures        []Picture             `json:"pictures,omitempty"`
	Background      string                `json:"background,omitempty"`
	RelatedAnime    []AnimeRelation       `json:"related_anime,omitempty"`
	RelatedManga    []MangaRelation       `json:"related_manga,omitempty"`
	Recommendations []MangaRecommendation `json:"recommendations,omitempty"`
	Serialization   []MagazineRole        `json:"serialization,omitempty"`
}

type RankingInfo struct {
	Rank         int  `json:"rank"`
	PreviousRank *int `json:"previous_rank,omitempty"`
}

type RankedAnime struct {
	Anime   Anime       `json:"node"`
	Ranking RankingInfo `json:"ranking"`
}

type RankedManga struct {
	Manga   Manga       `json:"node"`
	Ranking RankingInfo `json:"ranking"`
}

type AnimeListStatus struct {
	Status             AnimeListState `json:"status,omitempty"`
	Score              int            `json:"score"`
	NumEpisodesWatched int            `json:"num_episodes_watched"`
	IsRewatching       bool           `json:"is_rewatching"`
	StartDate          string         `json:"start_date,omitempty"`
	FinishDate         string         `json:"finish_date,omitempty"`
	Priority           int            `json:"priority"`
	NumTimesRewatched  int            `json:"num_times_rewatched"`
	RewatchValue       int            `json:"rewatch_value"`
	Tags               []string       `json:"tags,omitempty"`
	Comments           string         `json:"comments,omitempty"`
	UpdatedAt          time.Time      `json:"updated_at,omitempty"`
}

type MangaListStatus struct {
	Status          MangaListState `json:"status,omitempty"`
	Score           int            `json:"score"`
	NumVolumesRead  int            `json:"num_volumes_read"`
	NumChaptersRead int            `json:"num_chapters_read"`
	IsRereading     bool           `json:"is_rereading"`
	StartDate       string         `json:"start_date,omitempty"`
	FinishDate      string         `json:"finish_date,omitempty"`
	Priority        int            `json:"priority"`
	NumTimesReread  int            `json:"num_times_reread"`
	RereadValue     int            `json:"reread_value"`
	Tags            []string       `json:"tags,omitempty"`
	Comments        string         `json:"comments,omitempty"`
	UpdatedAt       time.Time      `json:"updated_at,omitempty"`
}

type AnimeListEntry struct {
	Anime      Anime           `json:"node"`
	ListStatus AnimeListStatus `json:"list_status"`
}

type MangaListEntry struct {
	Manga      Manga           `json:"node"`
	ListStatus MangaListStatus `json:"list_status"`
}

type UserAnimeStatistics struct {
	NumItemsWatching    int     `json:"num_items_watching"`
	NumItemsCompleted   int     `json:"num_items_completed"`
	NumItemsOnHold      int     `json:"num_items_on_hold"`
	NumItemsDropped     int     `json:"num_items_dropped"`
	NumItemsPlanToWatch int     `json:"num_items_plan_to_watch"`
	NumItems            int     `json:"num_items"`
	NumDaysWatched      float64 `json:"num_days_watched"`
	NumDaysWatching     float64 `json:"num_days_watching"`
	NumDaysCompleted    float64 `json:"num_days_completed"`
	NumDaysOnHold       float64 `json:"num_days_on_hold"`
	NumDaysDropped      float64 `json:"num_days_dropped"`
	NumDays             float64 `json:"num_days"`
	NumEpisodes         int     `json:"num_episodes"`
	NumTimesRewatched   int     `json:"num_times_rewatched"`
	MeanScore           float64 `json:"mean_score"`
}

type User struct {
	ID              int64                `json:"id"`
	Name            string               `json:"name"`
	Picture         string               `json:"picture,omitempty"`
	Gender          string               `json:"gender,omitempty"`
	Birthday        string               `json:"birthday,omitempty"`
	Location        string               `json:"location,omitempty"`
	JoinedAt        time.Time            `json:"joined_at,omitempty"`
	AnimeStatistics *UserAnimeStatistics `json:"anime_statistics,omitempty"`
	TimeZone        string               `json:"time_zone,omitempty"`
	IsSupporter     *bool                `json:"is_supporter,omitempty"`
}
