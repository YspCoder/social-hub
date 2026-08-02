package mixcloud

import "time"

// Pictures contains the documented Mixcloud image renditions.
type Pictures struct {
	Small        string `json:"small,omitempty"`
	Thumbnail    string `json:"thumbnail,omitempty"`
	MediumMobile string `json:"medium_mobile,omitempty"`
	Medium       string `json:"medium,omitempty"`
	Large        string `json:"large,omitempty"`
	Size320      string `json:"320wx320h,omitempty"`
	ExtraLarge   string `json:"extra_large,omitempty"`
	Size640      string `json:"640wx640h,omitempty"`
	Size768      string `json:"768wx768h,omitempty"`
	Size1024     string `json:"1024wx1024h,omitempty"`
}

// User is Mixcloud's profile representation.
type User struct {
	Key                 string            `json:"key"`
	URL                 string            `json:"url"`
	Name                string            `json:"name"`
	Username            string            `json:"username"`
	Pictures            Pictures          `json:"pictures"`
	Biography           string            `json:"biog,omitempty"`
	CreatedTime         time.Time         `json:"created_time,omitempty"`
	UpdatedTime         time.Time         `json:"updated_time,omitempty"`
	FollowerCount       *int64            `json:"follower_count,omitempty"`
	FollowingCount      *int64            `json:"following_count,omitempty"`
	CloudcastCount      *int64            `json:"cloudcast_count,omitempty"`
	FavoriteCount       *int64            `json:"favorite_count,omitempty"`
	ListenCount         *int64            `json:"listen_count,omitempty"`
	IsPro               bool              `json:"is_pro,omitempty"`
	IsPremium           bool              `json:"is_premium,omitempty"`
	City                string            `json:"city,omitempty"`
	Country             string            `json:"country,omitempty"`
	CoverPictures       map[string]string `json:"cover_pictures,omitempty"`
	PicturePrimaryColor string            `json:"picture_primary_color,omitempty"`
	Following           *bool             `json:"following,omitempty"`
}

// Tag is a Mixcloud genre or search tag.
type Tag struct {
	Key  string `json:"key"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// Section is one chapter or track in a Cloudcast timeline.
type Section struct {
	Artist    string `json:"artist,omitempty"`
	Song      string `json:"song,omitempty"`
	Chapter   string `json:"chapter,omitempty"`
	StartTime int    `json:"start_time"`
}

// Cloudcast is Mixcloud's audio show or track resource.
type Cloudcast struct {
	Key                 string    `json:"key"`
	URL                 string    `json:"url"`
	Name                string    `json:"name"`
	Description         string    `json:"description,omitempty"`
	Slug                string    `json:"slug"`
	Tags                []Tag     `json:"tags,omitempty"`
	CreatedTime         time.Time `json:"created_time,omitempty"`
	UpdatedTime         time.Time `json:"updated_time,omitempty"`
	PlayCount           *int64    `json:"play_count,omitempty"`
	FavoriteCount       *int64    `json:"favorite_count,omitempty"`
	CommentCount        *int64    `json:"comment_count,omitempty"`
	ListenerCount       *int64    `json:"listener_count,omitempty"`
	RepostCount         *int64    `json:"repost_count,omitempty"`
	Pictures            Pictures  `json:"pictures"`
	User                User      `json:"user"`
	Hosts               []User    `json:"hosts,omitempty"`
	AudioLength         int64     `json:"audio_length,omitempty"`
	Sections            []Section `json:"sections,omitempty"`
	PicturePrimaryColor string    `json:"picture_primary_color,omitempty"`
	IsPublic            *bool     `json:"is_public,omitempty"`
	Favorited           *bool     `json:"favorited,omitempty"`
	Reposted            *bool     `json:"reposted,omitempty"`
	ListenLater         *bool     `json:"listen_later,omitempty"`
}

// Comment is a public Mixcloud Cloudcast comment.
type Comment struct {
	Key        string    `json:"key"`
	URL        string    `json:"url"`
	User       User      `json:"user"`
	SubmitDate time.Time `json:"submit_date,omitempty"`
	Text       string    `json:"comment"`
}

// Paging contains the absolute links returned by Mixcloud list endpoints.
type Paging struct {
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

type CloudcastPage struct {
	Data   []Cloudcast `json:"data"`
	Paging Paging      `json:"paging,omitempty"`
	Name   string      `json:"name,omitempty"`
}

type UserPage struct {
	Data   []User `json:"data"`
	Paging Paging `json:"paging,omitempty"`
	Name   string `json:"name,omitempty"`
}

type TagPage struct {
	Data   []Tag  `json:"data"`
	Paging Paging `json:"paging,omitempty"`
	Name   string `json:"name,omitempty"`
}

type CommentPage struct {
	Data   []Comment `json:"data"`
	Paging Paging    `json:"paging,omitempty"`
	Name   string    `json:"name,omitempty"`
}

// PageRequest selects a Mixcloud connection page. Cursor is an offset emitted
// by this package; time filters apply only to date-ordered Cloudcast lists.
type PageRequest struct {
	Cursor     string
	MaxResults int
	StartTime  *time.Time
	EndTime    *time.Time
}

// SearchRequest selects one page of Mixcloud search results.
type SearchRequest struct {
	Query      string
	Cursor     string
	MaxResults int
}

// ActionResult is the result object returned by Mixcloud mutations.
type ActionResult struct {
	Key     string `json:"key,omitempty"`
	Message string `json:"message,omitempty"`
	Success bool   `json:"success"`
}

// ActionResponse preserves the mutation result and field-level details.
type ActionResponse struct {
	Details map[string][]string `json:"details,omitempty"`
	Result  *ActionResult       `json:"result,omitempty"`
}

// UploadRequest describes a new Cloudcast and its multipart sources.
type UploadRequest struct {
	Name            string
	AudioFilename   string
	AudioSize       int64
	Description     string
	Tags            []string
	Unlisted        *bool
	PublishDate     *time.Time
	DisableComments *bool
	HideStats       *bool
	Hosts           []string
	Sections        []Section
	PictureFilename string
	PictureMIME     string
	PictureSize     int64
}

// EditRequest contains optional Cloudcast metadata. A non-nil Tags, Sections,
// or Hosts slice replaces the complete corresponding collection.
type EditRequest struct {
	Name            *string
	Description     *string
	Tags            *[]string
	Unlisted        *bool
	Publish         bool
	Unpublish       bool
	PublishDate     *time.Time
	DisableComments *bool
	HideStats       *bool
	Hosts           *[]string
	Sections        *[]Section
	PictureFilename string
	PictureMIME     string
	PictureSize     int64
}
