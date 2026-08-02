package anilist

import "encoding/json"

type MediaType string

const (
	MediaAnime MediaType = "ANIME"
	MediaManga MediaType = "MANGA"
)

type MediaSort string

const (
	MediaSortSearchMatch    MediaSort = "SEARCH_MATCH"
	MediaSortTrendingDesc   MediaSort = "TRENDING_DESC"
	MediaSortPopularityDesc MediaSort = "POPULARITY_DESC"
	MediaSortScoreDesc      MediaSort = "SCORE_DESC"
	MediaSortStartDateDesc  MediaSort = "START_DATE_DESC"
)

type MediaSeason string

const (
	SeasonWinter MediaSeason = "WINTER"
	SeasonSpring MediaSeason = "SPRING"
	SeasonSummer MediaSeason = "SUMMER"
	SeasonFall   MediaSeason = "FALL"
)

type MediaListStatus string

const (
	ListCurrent   MediaListStatus = "CURRENT"
	ListPlanning  MediaListStatus = "PLANNING"
	ListCompleted MediaListStatus = "COMPLETED"
	ListDropped   MediaListStatus = "DROPPED"
	ListPaused    MediaListStatus = "PAUSED"
	ListRepeating MediaListStatus = "REPEATING"
)

type MediaListSort string

const (
	MediaListSortUpdatedDesc  MediaListSort = "UPDATED_TIME_DESC"
	MediaListSortScoreDesc    MediaListSort = "SCORE_DESC"
	MediaListSortProgressDesc MediaListSort = "PROGRESS_DESC"
	MediaListSortTitle        MediaListSort = "MEDIA_TITLE_ROMAJI"
)

type ActivityType string

const (
	ActivityText      ActivityType = "TEXT"
	ActivityAnimeList ActivityType = "ANIME_LIST"
	ActivityMangaList ActivityType = "MANGA_LIST"
	ActivityMediaList ActivityType = "MEDIA_LIST"
)

type LikeableType string

const (
	LikeActivity      LikeableType = "ACTIVITY"
	LikeActivityReply LikeableType = "ACTIVITY_REPLY"
)

type FuzzyDate struct {
	Year  int `json:"year,omitempty"`
	Month int `json:"month,omitempty"`
	Day   int `json:"day,omitempty"`
}

type MediaTitle struct {
	Romaji        string `json:"romaji,omitempty"`
	English       string `json:"english,omitempty"`
	Native        string `json:"native,omitempty"`
	UserPreferred string `json:"userPreferred,omitempty"`
}

type CoverImage struct {
	ExtraLarge string `json:"extraLarge,omitempty"`
	Large      string `json:"large,omitempty"`
	Medium     string `json:"medium,omitempty"`
	Color      string `json:"color,omitempty"`
}

type NextAiringEpisode struct {
	ID              int64 `json:"id"`
	AiringAt        int64 `json:"airingAt"`
	TimeUntilAiring int   `json:"timeUntilAiring"`
	Episode         int   `json:"episode"`
}

// Media is an AniList anime or manga catalog object.
type Media struct {
	ID                int64              `json:"id"`
	IDMal             *int64             `json:"idMal,omitempty"`
	Title             MediaTitle         `json:"title"`
	Type              MediaType          `json:"type"`
	Format            string             `json:"format,omitempty"`
	Status            string             `json:"status,omitempty"`
	Description       string             `json:"description,omitempty"`
	StartDate         FuzzyDate          `json:"startDate"`
	EndDate           FuzzyDate          `json:"endDate"`
	Season            MediaSeason        `json:"season,omitempty"`
	SeasonYear        int                `json:"seasonYear,omitempty"`
	SeasonInt         int                `json:"seasonInt,omitempty"`
	Episodes          *int               `json:"episodes,omitempty"`
	Duration          *int               `json:"duration,omitempty"`
	Chapters          *int               `json:"chapters,omitempty"`
	Volumes           *int               `json:"volumes,omitempty"`
	CountryOfOrigin   string             `json:"countryOfOrigin,omitempty"`
	IsLicensed        *bool              `json:"isLicensed,omitempty"`
	Source            string             `json:"source,omitempty"`
	CoverImage        CoverImage         `json:"coverImage"`
	BannerImage       string             `json:"bannerImage,omitempty"`
	Genres            []string           `json:"genres,omitempty"`
	Synonyms          []string           `json:"synonyms,omitempty"`
	AverageScore      *int               `json:"averageScore,omitempty"`
	MeanScore         *int               `json:"meanScore,omitempty"`
	Popularity        int                `json:"popularity,omitempty"`
	Favourites        int                `json:"favourites,omitempty"`
	Trending          int                `json:"trending,omitempty"`
	SiteURL           string             `json:"siteUrl,omitempty"`
	UpdatedAt         int64              `json:"updatedAt,omitempty"`
	NextAiringEpisode *NextAiringEpisode `json:"nextAiringEpisode,omitempty"`
}

type UserAvatar struct {
	Large  string `json:"large,omitempty"`
	Medium string `json:"medium,omitempty"`
}

// User is an AniList public profile or the authenticated Viewer.
type User struct {
	ID                      int64      `json:"id"`
	Name                    string     `json:"name"`
	About                   string     `json:"about,omitempty"`
	Avatar                  UserAvatar `json:"avatar"`
	BannerImage             string     `json:"bannerImage,omitempty"`
	SiteURL                 string     `json:"siteUrl,omitempty"`
	IsFollowing             *bool      `json:"isFollowing,omitempty"`
	IsFollower              *bool      `json:"isFollower,omitempty"`
	IsBlocked               *bool      `json:"isBlocked,omitempty"`
	UnreadNotificationCount *int       `json:"unreadNotificationCount,omitempty"`
	DonatorTier             int        `json:"donatorTier,omitempty"`
	DonatorBadge            string     `json:"donatorBadge,omitempty"`
	CreatedAt               int64      `json:"createdAt,omitempty"`
	UpdatedAt               int64      `json:"updatedAt,omitempty"`
}

// MediaListEntry is one anime or manga tracking record.
type MediaListEntry struct {
	ID                    int64           `json:"id"`
	UserID                int64           `json:"userId"`
	MediaID               int64           `json:"mediaId"`
	Status                MediaListStatus `json:"status,omitempty"`
	Score                 float64         `json:"score"`
	Progress              int             `json:"progress"`
	ProgressVolumes       int             `json:"progressVolumes"`
	Repeat                int             `json:"repeat"`
	Priority              int             `json:"priority"`
	Private               bool            `json:"private"`
	Notes                 string          `json:"notes,omitempty"`
	HiddenFromStatusLists bool            `json:"hiddenFromStatusLists"`
	CustomLists           json.RawMessage `json:"customLists,omitempty"`
	AdvancedScores        json.RawMessage `json:"advancedScores,omitempty"`
	StartedAt             FuzzyDate       `json:"startedAt"`
	CompletedAt           FuzzyDate       `json:"completedAt"`
	UpdatedAt             int64           `json:"updatedAt"`
	CreatedAt             int64           `json:"createdAt"`
	Media                 *Media          `json:"media,omitempty"`
}

// Activity represents the supported text and media-list activity union.
type Activity struct {
	Typename     string       `json:"__typename"`
	ID           int64        `json:"id"`
	UserID       int64        `json:"userId"`
	Type         ActivityType `json:"type"`
	ReplyCount   int          `json:"replyCount"`
	Text         *string      `json:"text,omitempty"`
	Status       *string      `json:"status,omitempty"`
	Progress     *string      `json:"progress,omitempty"`
	SiteURL      string       `json:"siteUrl,omitempty"`
	IsLocked     bool         `json:"isLocked"`
	IsSubscribed bool         `json:"isSubscribed"`
	LikeCount    int          `json:"likeCount"`
	IsLiked      bool         `json:"isLiked"`
	IsPinned     bool         `json:"isPinned"`
	CreatedAt    int64        `json:"createdAt"`
	User         *User        `json:"user,omitempty"`
	Media        *Media       `json:"media,omitempty"`
}

type ActivityReply struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"userId"`
	ActivityID int64  `json:"activityId"`
	Text       string `json:"text"`
	LikeCount  int    `json:"likeCount"`
	IsLiked    bool   `json:"isLiked"`
	CreatedAt  int64  `json:"createdAt"`
	User       *User  `json:"user,omitempty"`
}

type LikeResult struct {
	Typename  string `json:"__typename"`
	ID        int64  `json:"id"`
	LikeCount int    `json:"likeCount"`
	IsLiked   bool   `json:"isLiked"`
}
