package deviantart

import "encoding/json"

// User is DeviantArt's stable user representation.
type User struct {
	UserID        string       `json:"userid"`
	Username      string       `json:"username"`
	UserIcon      string       `json:"usericon"`
	Type          string       `json:"type"`
	Email         string       `json:"email,omitempty"`
	MatureContent bool         `json:"mature_content,omitempty"`
	Verified      bool         `json:"verified,omitempty"`
	Newsletter    bool         `json:"newsletter,omitempty"`
	IsMinor       bool         `json:"is_minor,omitempty"`
	UTID          string       `json:"utid,omitempty"`
	IsWatching    bool         `json:"is_watching,omitempty"`
	IsSubscribed  bool         `json:"is_subscribed,omitempty"`
	Details       *UserDetails `json:"details,omitempty"`
	Geo           *UserGeo     `json:"geo,omitempty"`
	Profile       *UserProfile `json:"profile,omitempty"`
	Stats         *UserStats   `json:"stats,omitempty"`
}

type UserDetails struct {
	Sex      *string `json:"sex"`
	Age      *int    `json:"age"`
	JoinDate string  `json:"joindate"`
}

type UserGeo struct {
	Country   string `json:"country"`
	CountryID int    `json:"countryid"`
	Timezone  string `json:"timezone"`
}

type UserProfile struct {
	IsArtist        bool    `json:"user_is_artist"`
	ArtistLevel     *string `json:"artist_level"`
	ArtistSpecialty *string `json:"artist_speciality"`
	RealName        string  `json:"real_name"`
	Tagline         string  `json:"tagline"`
	Website         string  `json:"website"`
	CoverPhoto      string  `json:"cover_photo"`
}

type UserStats struct {
	Watchers int64 `json:"watchers"`
	Friends  int64 `json:"friends"`
}

// Profile is the richer public profile response.
type Profile struct {
	User            User            `json:"user"`
	IsWatching      bool            `json:"is_watching"`
	ProfileURL      string          `json:"profile_url"`
	IsArtist        bool            `json:"user_is_artist"`
	ArtistLevel     *string         `json:"artist_level"`
	ArtistSpecialty *string         `json:"artist_specialty"`
	RealName        string          `json:"real_name"`
	Tagline         string          `json:"tagline"`
	CountryID       int             `json:"countryid"`
	Country         string          `json:"country"`
	Website         string          `json:"website"`
	Bio             string          `json:"bio"`
	CoverPhoto      *string         `json:"cover_photo"`
	LastStatus      json.RawMessage `json:"last_status"`
	Stats           ProfileStats    `json:"stats"`
}

type ProfileStats struct {
	UserDeviations   int64 `json:"user_deviations"`
	UserFavourites   int64 `json:"user_favourites"`
	UserComments     int64 `json:"user_comments"`
	ProfilePageViews int64 `json:"profile_pageviews"`
}

// Deviation is a published artwork, journal, literature item, or profile post.
type Deviation struct {
	DeviationID      string          `json:"deviationid"`
	PrintID          *string         `json:"printid"`
	URL              string          `json:"url,omitempty"`
	Title            string          `json:"title,omitempty"`
	IsFavourited     bool            `json:"is_favourited,omitempty"`
	IsDeleted        bool            `json:"is_deleted"`
	IsPublished      bool            `json:"is_published,omitempty"`
	IsBlocked        bool            `json:"is_blocked,omitempty"`
	Author           User            `json:"author,omitempty"`
	Stats            DeviationStats  `json:"stats,omitempty"`
	PublishedTime    string          `json:"published_time,omitempty"`
	AllowsComments   bool            `json:"allows_comments,omitempty"`
	FormattedExcerpt string          `json:"formatted_exerpt,omitempty"`
	SocialPreview    *ImageResource  `json:"social_preview,omitempty"`
	Preview          *ImageResource  `json:"preview,omitempty"`
	Content          *ImageResource  `json:"content,omitempty"`
	Thumbs           []ImageResource `json:"thumbs,omitempty"`
	Videos           []VideoResource `json:"videos,omitempty"`
	TextContent      *EditorText     `json:"text_content,omitempty"`
	Excerpt          string          `json:"excerpt,omitempty"`
	IsMature         bool            `json:"is_mature,omitempty"`
	IsDownloadable   bool            `json:"is_downloadable,omitempty"`
	DownloadFileSize int64           `json:"download_filesize,omitempty"`
}

type DeviationStats struct {
	Comments   int64 `json:"comments"`
	Favourites int64 `json:"favourites"`
}

type ImageResource struct {
	Src          string `json:"src"`
	Height       int    `json:"height"`
	Width        int    `json:"width"`
	Transparency bool   `json:"transparency"`
	FileSize     int64  `json:"filesize,omitempty"`
}

type VideoResource struct {
	Src      string `json:"src"`
	Quality  string `json:"quality"`
	FileSize int64  `json:"filesize"`
	Duration int64  `json:"duration"`
}

type EditorText struct {
	Excerpt string     `json:"excerpt"`
	Body    EditorBody `json:"body"`
}

type EditorBody struct {
	Type     string `json:"type"`
	Markup   string `json:"markup,omitempty"`
	Features string `json:"features"`
}

// Comment is a DeviantArt comment. ParentID preserves reply relationships.
type Comment struct {
	CommentID   string      `json:"commentid"`
	ParentID    *string     `json:"parentid"`
	Posted      string      `json:"posted"`
	Replies     int64       `json:"replies"`
	Hidden      *string     `json:"hidden"`
	Body        string      `json:"body"`
	IsLiked     bool        `json:"is_liked"`
	IsFeatured  bool        `json:"is_featured"`
	Likes       int64       `json:"likes"`
	User        User        `json:"user"`
	TextContent *EditorText `json:"text_content,omitempty"`
}

type DeviationPage struct {
	HasMore    bool        `json:"has_more"`
	NextOffset *int        `json:"next_offset"`
	Results    []Deviation `json:"results"`
}

type ProfilePostPage struct {
	HasMore    bool        `json:"has_more"`
	NextCursor *string     `json:"next_cursor"`
	PrevCursor *string     `json:"prev_cursor"`
	Results    []Deviation `json:"results"`
}

type CommentPage struct {
	HasMore    bool      `json:"has_more"`
	NextOffset *int      `json:"next_offset"`
	HasLess    bool      `json:"has_less"`
	PrevOffset *int      `json:"prev_offset"`
	Total      int64     `json:"total"`
	Thread     []Comment `json:"thread"`
}

type StatusPublishResponse struct {
	StatusID string `json:"statusid"`
}

type FavouriteResponse struct {
	Success    bool  `json:"success"`
	Favourites int64 `json:"favourites"`
}

type StatusPostRequest struct {
	Body          string
	ShareID       string
	ShareParentID string
	StashID       string
}

type GalleryPageRequest struct {
	Username   string
	Offset     int
	MaxResults int
}

type ProfilePostsRequest struct {
	Username string
	Cursor   string
}

type DeviationCommentsRequest struct {
	DeviationID string
	CommentID   string
	MaxDepth    int
	Offset      int
	MaxResults  int
}

type DeviationCommentRequest struct {
	DeviationID string
	ParentID    string
	Body        string
}

type FavouriteRequest struct {
	DeviationID string
	FolderIDs   []string
}
