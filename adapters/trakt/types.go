package trakt

import (
	"encoding/json"
	"time"
)

// MediaType identifies a Trakt media resource.
type MediaType string

const (
	MediaMovie   MediaType = "movie"
	MediaShow    MediaType = "show"
	MediaSeason  MediaType = "season"
	MediaEpisode MediaType = "episode"
	MediaPerson  MediaType = "person"
	MediaList    MediaType = "list"
)

// IDs contains Trakt and supported external identifiers.
type IDs struct {
	Trakt int64  `json:"trakt,omitempty"`
	Slug  string `json:"slug,omitempty"`
	IMDB  string `json:"imdb,omitempty"`
	TMDB  int64  `json:"tmdb,omitempty"`
	TVDB  int64  `json:"tvdb,omitempty"`
}

type Images struct {
	Fanart     []string `json:"fanart,omitempty"`
	Poster     []string `json:"poster,omitempty"`
	Logo       []string `json:"logo,omitempty"`
	Clearart   []string `json:"clearart,omitempty"`
	Banner     []string `json:"banner,omitempty"`
	Thumb      []string `json:"thumb,omitempty"`
	Screenshot []string `json:"screenshot,omitempty"`
}

type Movie struct {
	Title                 string     `json:"title"`
	Year                  int        `json:"year,omitempty"`
	IDs                   IDs        `json:"ids"`
	Images                Images     `json:"images,omitempty"`
	Tagline               string     `json:"tagline,omitempty"`
	Overview              string     `json:"overview,omitempty"`
	Released              string     `json:"released,omitempty"`
	Runtime               int        `json:"runtime,omitempty"`
	Country               string     `json:"country,omitempty"`
	Status                string     `json:"status,omitempty"`
	Rating                float64    `json:"rating,omitempty"`
	Votes                 int64      `json:"votes,omitempty"`
	CommentCount          int        `json:"comment_count,omitempty"`
	Trailer               string     `json:"trailer,omitempty"`
	Homepage              string     `json:"homepage,omitempty"`
	UpdatedAt             *time.Time `json:"updated_at,omitempty"`
	Language              string     `json:"language,omitempty"`
	Languages             []string   `json:"languages,omitempty"`
	AvailableTranslations []string   `json:"available_translations,omitempty"`
	Genres                []string   `json:"genres,omitempty"`
	Subgenres             []string   `json:"subgenres,omitempty"`
	Certification         string     `json:"certification,omitempty"`
	OriginalTitle         string     `json:"original_title,omitempty"`
}

type AirSchedule struct {
	Day      string `json:"day,omitempty"`
	Time     string `json:"time,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type Show struct {
	Title                 string       `json:"title"`
	Year                  int          `json:"year,omitempty"`
	IDs                   IDs          `json:"ids"`
	Images                Images       `json:"images,omitempty"`
	AiredEpisodes         int          `json:"aired_episodes,omitempty"`
	Tagline               string       `json:"tagline,omitempty"`
	Overview              string       `json:"overview,omitempty"`
	FirstAired            *time.Time   `json:"first_aired,omitempty"`
	Airs                  *AirSchedule `json:"airs,omitempty"`
	Runtime               int          `json:"runtime,omitempty"`
	TotalRuntime          int          `json:"total_runtime,omitempty"`
	Certification         string       `json:"certification,omitempty"`
	Network               string       `json:"network,omitempty"`
	Country               string       `json:"country,omitempty"`
	Status                string       `json:"status,omitempty"`
	Rating                float64      `json:"rating,omitempty"`
	Votes                 int64        `json:"votes,omitempty"`
	CommentCount          int          `json:"comment_count,omitempty"`
	Trailer               string       `json:"trailer,omitempty"`
	Homepage              string       `json:"homepage,omitempty"`
	UpdatedAt             *time.Time   `json:"updated_at,omitempty"`
	Language              string       `json:"language,omitempty"`
	Languages             []string     `json:"languages,omitempty"`
	AvailableTranslations []string     `json:"available_translations,omitempty"`
	Genres                []string     `json:"genres,omitempty"`
	Subgenres             []string     `json:"subgenres,omitempty"`
	OriginalTitle         string       `json:"original_title,omitempty"`
}

type Episode struct {
	Season                int        `json:"season"`
	Number                int        `json:"number"`
	Title                 string     `json:"title,omitempty"`
	FirstAired            *time.Time `json:"first_aired,omitempty"`
	Released              string     `json:"released,omitempty"`
	EffectiveReleaseDate  string     `json:"effective_release_date,omitempty"`
	AbsoluteNumber        int        `json:"number_abs,omitempty"`
	Rating                float64    `json:"rating,omitempty"`
	Votes                 int64      `json:"votes,omitempty"`
	CommentCount          int        `json:"comment_count,omitempty"`
	UpdatedAt             *time.Time `json:"updated_at,omitempty"`
	AvailableTranslations []string   `json:"available_translations,omitempty"`
	Runtime               int        `json:"runtime,omitempty"`
	Overview              string     `json:"overview,omitempty"`
	EpisodeType           string     `json:"episode_type,omitempty"`
	IDs                   IDs        `json:"ids"`
	OriginalTitle         string     `json:"original_title,omitempty"`
	Images                Images     `json:"images,omitempty"`
}

type Person struct {
	Name       string `json:"name"`
	IDs        IDs    `json:"ids"`
	Biography  string `json:"biography,omitempty"`
	Birthday   string `json:"birthday,omitempty"`
	Death      string `json:"death,omitempty"`
	Birthplace string `json:"birthplace,omitempty"`
	Homepage   string `json:"homepage,omitempty"`
}

type SearchResult struct {
	Score   float64         `json:"score"`
	Type    MediaType       `json:"type"`
	Movie   *Movie          `json:"movie,omitempty"`
	Show    *Show           `json:"show,omitempty"`
	Episode *Episode        `json:"episode,omitempty"`
	Person  *Person         `json:"person,omitempty"`
	List    json.RawMessage `json:"list,omitempty"`
}

type MovieTrend struct {
	Watchers int64 `json:"watchers"`
	Movie    Movie `json:"movie"`
}

type ShowTrend struct {
	Watchers int64 `json:"watchers"`
	Show     Show  `json:"show"`
}

type Profile struct {
	Username string     `json:"username"`
	Private  bool       `json:"private"`
	Deleted  bool       `json:"deleted"`
	Name     string     `json:"name,omitempty"`
	VIP      bool       `json:"vip,omitempty"`
	Director bool       `json:"director,omitempty"`
	IDs      IDs        `json:"ids"`
	JoinedAt *time.Time `json:"joined_at,omitempty"`
	Location string     `json:"location,omitempty"`
	About    string     `json:"about,omitempty"`
	Gender   string     `json:"gender,omitempty"`
	Age      int        `json:"age,omitempty"`
	Images   struct {
		Avatar struct {
			Full string `json:"full"`
		} `json:"avatar"`
	} `json:"images,omitempty"`
}

type Permissions struct {
	Commenting bool `json:"commenting"`
	Liking     bool `json:"liking"`
	Following  bool `json:"following"`
}

type UserAccountPreferences struct {
	Timezone   string `json:"timezone"`
	DateFormat string `json:"date_format"`
	Time24Hour bool   `json:"time_24hr"`
}

type AccountLimits struct {
	List struct {
		Count     int `json:"count"`
		ItemCount int `json:"item_count"`
	} `json:"list"`
	Watchlist struct {
		ItemCount int `json:"item_count"`
	} `json:"watchlist"`
	Favorites struct {
		ItemCount int `json:"item_count"`
	} `json:"favorites"`
}

type UserSettings struct {
	User        Profile                `json:"user"`
	Permissions Permissions            `json:"permissions"`
	Account     UserAccountPreferences `json:"account"`
	Limits      *AccountLimits         `json:"limits,omitempty"`
}

type HistoryItem struct {
	ID        int64     `json:"id"`
	WatchedAt time.Time `json:"watched_at"`
	Action    string    `json:"action"`
	Type      MediaType `json:"type"`
	Movie     *Movie    `json:"movie,omitempty"`
	Episode   *Episode  `json:"episode,omitempty"`
	Show      *Show     `json:"show,omitempty"`
}

type WatchlistItem struct {
	Rank     int             `json:"rank,omitempty"`
	ID       int64           `json:"id,omitempty"`
	ListedAt time.Time       `json:"listed_at"`
	Type     MediaType       `json:"type"`
	Movie    *Movie          `json:"movie,omitempty"`
	Show     *Show           `json:"show,omitempty"`
	Season   json.RawMessage `json:"season,omitempty"`
	Episode  *Episode        `json:"episode,omitempty"`
}

type RatingItem struct {
	RatedAt time.Time `json:"rated_at"`
	Rating  int       `json:"rating"`
	Type    MediaType `json:"type"`
	Movie   *Movie    `json:"movie,omitempty"`
	Show    *Show     `json:"show,omitempty"`
	Episode *Episode  `json:"episode,omitempty"`
}

type CommentUserStats struct {
	Rating         *int `json:"rating,omitempty"`
	PlayCount      int  `json:"play_count"`
	CompletedCount int  `json:"completed_count"`
}

type Comment struct {
	ID         int64            `json:"id"`
	ParentID   int64            `json:"parent_id"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Text       string           `json:"comment"`
	Spoiler    bool             `json:"spoiler"`
	Review     bool             `json:"review"`
	Replies    int              `json:"replies"`
	Likes      int              `json:"likes"`
	Language   string           `json:"language,omitempty"`
	UserRating *int             `json:"user_rating,omitempty"`
	UserStats  CommentUserStats `json:"user_stats"`
	User       Profile          `json:"user"`
}
