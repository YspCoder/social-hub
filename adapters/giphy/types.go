package giphy

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// ContentType selects GIF or Sticker endpoints.
type ContentType string

const (
	ContentGIF     ContentType = "gifs"
	ContentSticker ContentType = "stickers"
)

// Rating is a GIPHY content rating filter.
type Rating string

const (
	RatingG    Rating = "g"
	RatingPG   Rating = "pg"
	RatingPG13 Rating = "pg-13"
	RatingR    Rating = "r"
)

// CommonOptions are request dimensions shared by discovery endpoints.
type CommonOptions struct {
	CustomerID        string
	Rating            Rating
	CountryCode       string
	Region            string
	Bundle            string
	RemoveLowContrast *bool
}

// SearchRequest describes a GIF or Sticker search.
type SearchRequest struct {
	Content    ContentType
	Query      string
	Limit      int
	Offset     int
	Language   string
	ChannelIDs []int64
	CommonOptions
}

// TrendingRequest describes a current trending page.
type TrendingRequest struct {
	Content ContentType
	Limit   int
	Offset  int
	CommonOptions
}

// TranslateRequest selects the single GIF or Sticker matching a phrase.
type TranslateRequest struct {
	Content ContentType
	Query   string
	CommonOptions
}

// RandomRequest selects a random GIF or Sticker.
type RandomRequest struct {
	Content ContentType
	Tag     string
	CommonOptions
}

// GetRequest selects one GIF by ID.
type GetRequest struct {
	ID string
	CommonOptions
}

// GetManyRequest selects up to 100 GIFs by ID.
type GetManyRequest struct {
	IDs []string
	CommonOptions
}

// TermRequest selects an autocomplete page.
type TermRequest struct {
	Query      string
	Limit      int
	Offset     int
	CustomerID string
}

// UploadRequest describes a streaming animated GIF or video upload.
type UploadRequest struct {
	Filename      string
	MIME          string
	Size          int64
	Username      string
	Tags          []string
	SourcePostURL string
	CustomerID    string
	CountryCode   string
	Region        string
}

// AnalyticsEvent selects one response-provided tracking URL.
type AnalyticsEvent string

const (
	AnalyticsView  AnalyticsEvent = "onload"
	AnalyticsClick AnalyticsEvent = "onclick"
	AnalyticsSend  AnalyticsEvent = "onsent"
)

// AnalyticsRequest registers a user action at a response-provided tracking URL.
type AnalyticsRequest struct {
	TrackingURL string
	CustomerID  string
	Timestamp   time.Time
}

// GIF is the stable subset of a GIPHY GIF, Sticker, or Emoji object.
type GIF struct {
	Type             string               `json:"type"`
	ID               string               `json:"id"`
	Slug             string               `json:"slug"`
	URL              string               `json:"url"`
	BitlyURL         string               `json:"bitly_url"`
	EmbedURL         string               `json:"embed_url"`
	Username         string               `json:"username"`
	Source           string               `json:"source"`
	Rating           string               `json:"rating"`
	ContentURL       string               `json:"content_url"`
	SourceTLD        string               `json:"source_tld"`
	SourcePostURL    string               `json:"source_post_url"`
	UpdateDatetime   string               `json:"update_datetime"`
	CreateDatetime   string               `json:"create_datetime"`
	ImportDatetime   string               `json:"import_datetime"`
	TrendingDatetime string               `json:"trending_datetime"`
	Title            string               `json:"title"`
	AltText          string               `json:"alt_text"`
	IsLowContrast    bool                 `json:"is_low_contrast"`
	VariationCount   int                  `json:"variation_count"`
	User             *User                `json:"user"`
	Images           map[string]Rendition `json:"images"`
	Analytics        Analytics            `json:"analytics"`
}

// Rendition contains one GIPHY media representation.
type Rendition struct {
	URL      string `json:"url"`
	Width    Number `json:"width"`
	Height   Number `json:"height"`
	Size     Number `json:"size"`
	Frames   Number `json:"frames"`
	MP4      string `json:"mp4"`
	MP4Size  Number `json:"mp4_size"`
	WebP     string `json:"webp"`
	WebPSize Number `json:"webp_size"`
	Hash     string `json:"hash"`
}

// Number decodes numeric GIPHY fields that are returned as strings or numbers.
type Number int64

// User is the public attribution embedded in a GIPHY object.
type User struct {
	AvatarURL    string `json:"avatar_url"`
	BannerURL    string `json:"banner_url"`
	ProfileURL   string `json:"profile_url"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	InstagramURL string `json:"instagram_url"`
	WebsiteURL   string `json:"website_url"`
	IsVerified   bool   `json:"is_verified"`
}

// Analytics contains the response-provided action registration URLs.
type Analytics struct {
	OnLoad  AnalyticsURL `json:"onload"`
	OnClick AnalyticsURL `json:"onclick"`
	OnSent  AnalyticsURL `json:"onsent"`
}

// AnalyticsURL is one GIPHY action registration URL.
type AnalyticsURL struct {
	URL string `json:"url"`
}

// TrackingURL returns the URL for one analytics event.
func (gif GIF) TrackingURL(event AnalyticsEvent) string {
	switch event {
	case AnalyticsView:
		return gif.Analytics.OnLoad.URL
	case AnalyticsClick:
		return gif.Analytics.OnClick.URL
	case AnalyticsSend:
		return gif.Analytics.OnSent.URL
	default:
		return ""
	}
}

// Meta is GIPHY's response status and correlation metadata.
type Meta struct {
	Status     int    `json:"status"`
	Message    string `json:"msg"`
	ResponseID string `json:"response_id"`
}

// Pagination describes GIPHY offset pagination.
type Pagination struct {
	Offset     int `json:"offset"`
	TotalCount int `json:"total_count"`
	Count      int `json:"count"`
}

// Page contains one typed GIPHY result page.
type Page[T any] struct {
	Items      []T
	Pagination Pagination
	ResponseID string
}

// Category is a GIPHY GIF category.
type Category struct {
	Name          string        `json:"name"`
	NameEncoded   string        `json:"name_encoded"`
	Subcategories []Subcategory `json:"subcategories"`
	GIF           *GIF          `json:"gif"`
}

// Subcategory is one GIPHY category child.
type Subcategory struct {
	Name        string `json:"name"`
	NameEncoded string `json:"name_encoded"`
}

// Term is an autocomplete or related-search term.
type Term struct {
	Name string `json:"name"`
}

// UploadResult identifies newly uploaded GIPHY media.
type UploadResult struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Slug string `json:"slug"`
}

// DiscoveryWorkflow exposes GIPHY media discovery endpoints.
type DiscoveryWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (Page[GIF], error)
	Trending(context.Context, TrendingRequest, ...socialhub.CallOption) (Page[GIF], error)
	Translate(context.Context, TranslateRequest, ...socialhub.CallOption) (*GIF, error)
	Random(context.Context, RandomRequest, ...socialhub.CallOption) (*GIF, error)
	Get(context.Context, GetRequest, ...socialhub.CallOption) (*GIF, error)
	GetMany(context.Context, GetManyRequest, ...socialhub.CallOption) (Page[GIF], error)
	RandomID(context.Context, ...socialhub.CallOption) (string, error)
	Categories(context.Context, string, ...socialhub.CallOption) (Page[Category], error)
	Autocomplete(context.Context, TermRequest, ...socialhub.CallOption) ([]Term, error)
	Related(context.Context, string, string, ...socialhub.CallOption) ([]Term, error)
	TrendingSearches(context.Context, string, ...socialhub.CallOption) ([]string, error)
}

// UploadWorkflow exposes GIPHY's streaming upload endpoint.
type UploadWorkflow interface {
	Upload(context.Context, UploadRequest, io.Reader, ...socialhub.CallOption) (*UploadResult, error)
}

// AnalyticsWorkflow exposes GIPHY action registration.
type AnalyticsWorkflow interface {
	Register(context.Context, AnalyticsRequest, ...socialhub.CallOption) error
}
