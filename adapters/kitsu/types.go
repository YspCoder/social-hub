package kitsu

import (
	"encoding/json"
	"time"
)

type MediaKind string

const (
	MediaAnime MediaKind = "Anime"
	MediaManga MediaKind = "Manga"
)

type LibraryStatus string

const (
	LibraryCurrent   LibraryStatus = "current"
	LibraryPlanned   LibraryStatus = "planned"
	LibraryCompleted LibraryStatus = "completed"
	LibraryOnHold    LibraryStatus = "on_hold"
	LibraryDropped   LibraryStatus = "dropped"
)

type ImageDimensions struct {
	Tiny     string `json:"tiny,omitempty"`
	Small    string `json:"small,omitempty"`
	Medium   string `json:"medium,omitempty"`
	Large    string `json:"large,omitempty"`
	Original string `json:"original,omitempty"`
}

// Media contains fields shared by Kitsu anime and manga resources.
type Media struct {
	ID                string                     `json:"id"`
	Kind              MediaKind                  `json:"kind"`
	CanonicalTitle    string                     `json:"canonicalTitle"`
	Titles            map[string]string          `json:"titles,omitempty"`
	AbbreviatedTitles []string                   `json:"abbreviatedTitles,omitempty"`
	Synopsis          string                     `json:"synopsis,omitempty"`
	AverageRating     string                     `json:"averageRating,omitempty"`
	RatingRank        int                        `json:"ratingRank,omitempty"`
	PopularityRank    int                        `json:"popularityRank,omitempty"`
	UserCount         int                        `json:"userCount,omitempty"`
	FavoritesCount    int                        `json:"favoritesCount,omitempty"`
	StartDate         string                     `json:"startDate,omitempty"`
	EndDate           string                     `json:"endDate,omitempty"`
	NextRelease       *time.Time                 `json:"nextRelease,omitempty"`
	Subtype           string                     `json:"subtype,omitempty"`
	Status            string                     `json:"status,omitempty"`
	AgeRating         string                     `json:"ageRating,omitempty"`
	AgeRatingGuide    string                     `json:"ageRatingGuide,omitempty"`
	PosterImage       ImageDimensions            `json:"posterImage,omitempty"`
	CoverImage        ImageDimensions            `json:"coverImage,omitempty"`
	NSFW              bool                       `json:"nsfw"`
	EpisodeCount      int                        `json:"episodeCount,omitempty"`
	EpisodeLength     int                        `json:"episodeLength,omitempty"`
	TotalLength       int                        `json:"totalLength,omitempty"`
	YouTubeVideoID    string                     `json:"youtubeVideoId,omitempty"`
	ChapterCount      int                        `json:"chapterCount,omitempty"`
	VolumeCount       int                        `json:"volumeCount,omitempty"`
	Serialization     string                     `json:"serialization,omitempty"`
	CreatedAt         time.Time                  `json:"createdAt,omitempty"`
	UpdatedAt         time.Time                  `json:"updatedAt,omitempty"`
	Extensions        map[string]json.RawMessage `json:"extensions,omitempty"`
}

type User struct {
	ID             string          `json:"id"`
	Name           string          `json:"name,omitempty"`
	Slug           string          `json:"slug,omitempty"`
	About          string          `json:"about,omitempty"`
	Location       string          `json:"location,omitempty"`
	Website        string          `json:"website,omitempty"`
	Birthday       string          `json:"birthday,omitempty"`
	Gender         string          `json:"gender,omitempty"`
	Avatar         ImageDimensions `json:"avatar,omitempty"`
	CoverImage     ImageDimensions `json:"coverImage,omitempty"`
	FollowersCount int             `json:"followersCount,omitempty"`
	FollowingCount int             `json:"followingCount,omitempty"`
	PostsCount     int             `json:"postsCount,omitempty"`
	CommentsCount  int             `json:"commentsCount,omitempty"`
	CreatedAt      time.Time       `json:"createdAt,omitempty"`
	UpdatedAt      time.Time       `json:"updatedAt,omitempty"`
}

type LibraryEntry struct {
	ID              string        `json:"id"`
	UserID          string        `json:"userId"`
	MediaID         string        `json:"mediaId"`
	MediaKind       MediaKind     `json:"mediaKind"`
	Status          LibraryStatus `json:"status"`
	Progress        int           `json:"progress"`
	VolumesOwned    int           `json:"volumesOwned"`
	Reconsuming     bool          `json:"reconsuming"`
	ReconsumeCount  int           `json:"reconsumeCount"`
	Notes           string        `json:"notes,omitempty"`
	Private         bool          `json:"private"`
	ReactionSkipped string        `json:"reactionSkipped,omitempty"`
	RatingTwenty    *int          `json:"ratingTwenty,omitempty"`
	ProgressedAt    *time.Time    `json:"progressedAt,omitempty"`
	StartedAt       *time.Time    `json:"startedAt,omitempty"`
	FinishedAt      *time.Time    `json:"finishedAt,omitempty"`
	CreatedAt       time.Time     `json:"createdAt,omitempty"`
	UpdatedAt       time.Time     `json:"updatedAt,omitempty"`
	Media           *Media        `json:"media,omitempty"`
}

type Post struct {
	ID               string          `json:"id"`
	UserID           string          `json:"userId"`
	Content          string          `json:"content"`
	ContentFormatted string          `json:"contentFormatted,omitempty"`
	CommentsCount    int             `json:"commentsCount"`
	PostLikesCount   int             `json:"postLikesCount"`
	Spoiler          bool            `json:"spoiler"`
	NSFW             bool            `json:"nsfw"`
	Blocked          bool            `json:"blocked"`
	DeletedAt        *time.Time      `json:"deletedAt,omitempty"`
	EditedAt         *time.Time      `json:"editedAt,omitempty"`
	TargetInterest   string          `json:"targetInterest,omitempty"`
	Embed            json.RawMessage `json:"embed,omitempty"`
	EmbedURL         string          `json:"embedUrl,omitempty"`
	CreatedAt        time.Time       `json:"createdAt,omitempty"`
	UpdatedAt        time.Time       `json:"updatedAt,omitempty"`
	User             *User           `json:"user,omitempty"`
}

type Comment struct {
	ID               string          `json:"id"`
	PostID           string          `json:"postId"`
	UserID           string          `json:"userId"`
	ParentID         string          `json:"parentId,omitempty"`
	Content          string          `json:"content"`
	ContentFormatted string          `json:"contentFormatted,omitempty"`
	Blocked          bool            `json:"blocked"`
	DeletedAt        *time.Time      `json:"deletedAt,omitempty"`
	LikesCount       int             `json:"likesCount"`
	RepliesCount     int             `json:"repliesCount"`
	EditedAt         *time.Time      `json:"editedAt,omitempty"`
	Embed            json.RawMessage `json:"embed,omitempty"`
	EmbedURL         string          `json:"embedUrl,omitempty"`
	CreatedAt        time.Time       `json:"createdAt,omitempty"`
	UpdatedAt        time.Time       `json:"updatedAt,omitempty"`
	User             *User           `json:"user,omitempty"`
}
