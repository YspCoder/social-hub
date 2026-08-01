package wordpresscom

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// PostStatus is a WordPress publication state.
type PostStatus string

const (
	PostPublished PostStatus = "publish"
	PostPrivate   PostStatus = "private"
	PostDraft     PostStatus = "draft"
	PostPending   PostStatus = "pending"
	PostFuture    PostStatus = "future"
)

// PostWriteRequest preserves WordPress-specific post fields absent from the common model.
type PostWriteRequest struct {
	Title            *string
	Content          *string
	Excerpt          *string
	Slug             *string
	Status           *PostStatus
	Date             *time.Time
	Categories       []string
	Tags             []string
	FeaturedImageID  *string
	Publicize        *bool
	PublicizeMessage *string
	CommentsOpen     *bool
	LikesEnabled     *bool
}

// PostWorkflow manages rich WordPress Posts and publication states.
type PostWorkflow interface {
	CreatePost(context.Context, PostWriteRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	UpdatePost(context.Context, string, PostWriteRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	RestorePost(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
}

// Site is the stable subset of WordPress.com site metadata.
type Site struct {
	ID               int64           `json:"ID"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	URL              string          `json:"URL"`
	Jetpack          bool            `json:"jetpack"`
	Private          bool            `json:"is_private"`
	Visible          bool            `json:"visible"`
	PostCount        int64           `json:"post_count"`
	SubscribersCount int64           `json:"subscribers_count"`
	Raw              json.RawMessage `json:"-"`
}

func (site *Site) UnmarshalJSON(data []byte) error {
	type alias Site
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*site = Site(decoded)
	site.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// SiteWorkflow reads the configured site identity and capabilities.
type SiteWorkflow interface {
	GetSite(context.Context, ...socialhub.CallOption) (*Site, error)
}

// MediaLibraryWorkflow adds permanent media deletion to the common uploader.
type MediaLibraryWorkflow interface {
	DeleteMedia(context.Context, string, ...socialhub.CallOption) error
}

type wpUser struct {
	ID          int64           `json:"ID"`
	Username    string          `json:"username"`
	Login       string          `json:"login"`
	DisplayName string          `json:"display_name"`
	Name        string          `json:"name"`
	AvatarURL   string          `json:"avatar_URL"`
	ProfileURL  string          `json:"profile_URL"`
	URL         string          `json:"URL"`
	Raw         json.RawMessage `json:"-"`
}

func (user *wpUser) UnmarshalJSON(data []byte) error {
	type alias wpUser
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*user = wpUser(decoded)
	user.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type wpPost struct {
	ID            int64              `json:"ID"`
	SiteID        int64              `json:"site_ID"`
	Author        wpUser             `json:"author"`
	Date          *time.Time         `json:"date"`
	Modified      *time.Time         `json:"modified"`
	Title         string             `json:"title"`
	URL           string             `json:"URL"`
	ShortURL      string             `json:"short_URL"`
	Content       string             `json:"content"`
	Excerpt       string             `json:"excerpt"`
	Slug          string             `json:"slug"`
	Status        string             `json:"status"`
	Type          string             `json:"type"`
	GlobalID      string             `json:"global_ID"`
	FeaturedImage string             `json:"featured_image"`
	CommentCount  int64              `json:"comment_count"`
	LikeCount     int64              `json:"like_count"`
	ILike         bool               `json:"i_like"`
	Attachments   map[string]wpMedia `json:"attachments"`
	Raw           json.RawMessage    `json:"-"`
}

func (post *wpPost) UnmarshalJSON(data []byte) error {
	type alias wpPost
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*post = wpPost(decoded)
	post.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type wpReference struct {
	ID int64 `json:"ID"`
}

type wpComment struct {
	ID         int64           `json:"ID"`
	Post       json.RawMessage `json:"post"`
	Author     wpUser          `json:"author"`
	Date       *time.Time      `json:"date"`
	URL        string          `json:"URL"`
	Content    string          `json:"content"`
	RawContent string          `json:"raw_content"`
	Status     string          `json:"status"`
	Parent     json.RawMessage `json:"parent"`
	Type       string          `json:"type"`
	LikeCount  int64           `json:"like_count"`
	ILike      bool            `json:"i_like"`
	Raw        json.RawMessage `json:"-"`
}

func (comment *wpComment) UnmarshalJSON(data []byte) error {
	type alias wpComment
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*comment = wpComment(decoded)
	comment.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type wpMedia struct {
	ID          int64           `json:"ID"`
	Date        *time.Time      `json:"date"`
	URL         string          `json:"URL"`
	GUID        string          `json:"guid"`
	File        string          `json:"file"`
	Extension   string          `json:"extension"`
	MIME        string          `json:"mime_type"`
	Title       string          `json:"title"`
	Caption     string          `json:"caption"`
	Description string          `json:"description"`
	Alt         string          `json:"alt"`
	Width       int             `json:"width"`
	Height      int             `json:"height"`
	Size        int64           `json:"size"`
	Length      int64           `json:"length"`
	Raw         json.RawMessage `json:"-"`
}

func (media *wpMedia) UnmarshalJSON(data []byte) error {
	type alias wpMedia
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*media = wpMedia(decoded)
	media.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type postListResponse struct {
	Found int64    `json:"found"`
	Posts []wpPost `json:"posts"`
	Meta  struct {
		NextPage string `json:"next_page"`
	} `json:"meta"`
}

type commentListResponse struct {
	Found    int64       `json:"found"`
	SiteID   int64       `json:"site_ID"`
	Comments []wpComment `json:"comments"`
}

type mediaUploadResponse struct {
	Media  []wpMedia `json:"media"`
	Errors []struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	} `json:"errors"`
}

type likeResponse struct {
	Success   bool  `json:"success"`
	ILike     bool  `json:"i_like"`
	LikeCount int64 `json:"like_count"`
	SiteID    int64 `json:"site_ID"`
	PostID    int64 `json:"post_ID"`
}

type uploadState struct {
	request   socialhub.BeginUploadRequest
	media     *socialhub.Media
	uploading bool
}

var _ PostWorkflow = (*Client)(nil)
var _ SiteWorkflow = (*Client)(nil)
var _ MediaLibraryWorkflow = (*Client)(nil)
