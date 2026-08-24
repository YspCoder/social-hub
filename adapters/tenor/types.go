package tenor

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	MaximumPageSize = 50
	MaximumPostIDs  = 50
)

// SafetyFilter selects Tenor's Motion Picture Association-aligned content
// filter. An empty value omits the parameter and uses Tenor's off default.
type SafetyFilter string

const (
	SafetyOff    SafetyFilter = "off"
	SafetyLow    SafetyFilter = "low"
	SafetyMedium SafetyFilter = "medium"
	SafetyHigh   SafetyFilter = "high"
)

// ContentKind selects GIFs or one of Tenor's documented sticker subsets.
type ContentKind string

const (
	ContentGIF             ContentKind = ""
	ContentSticker         ContentKind = "sticker"
	ContentAnimatedSticker ContentKind = "sticker,-static"
	ContentStaticSticker   ContentKind = "sticker,static"
)

// AspectRatioRange constrains discovery result aspect ratios.
type AspectRatioRange string

const (
	AspectRatioAll      AspectRatioRange = "all"
	AspectRatioWide     AspectRatioRange = "wide"
	AspectRatioStandard AspectRatioRange = "standard"
)

// CategoryType selects featured reaction categories or trending terms.
type CategoryType string

const (
	CategoryFeatured CategoryType = "featured"
	CategoryTrending CategoryType = "trending"
)

// MediaFormatName is one content format currently documented by Tenor.
type MediaFormatName string

const (
	FormatPreview             MediaFormatName = "preview"
	FormatGIF                 MediaFormatName = "gif"
	FormatMediumGIF           MediaFormatName = "mediumgif"
	FormatTinyGIF             MediaFormatName = "tinygif"
	FormatNanoGIF             MediaFormatName = "nanogif"
	FormatMP4                 MediaFormatName = "mp4"
	FormatLoopedMP4           MediaFormatName = "loopedmp4"
	FormatTinyMP4             MediaFormatName = "tinymp4"
	FormatNanoMP4             MediaFormatName = "nanomp4"
	FormatWebM                MediaFormatName = "webm"
	FormatTinyWebM            MediaFormatName = "tinywebm"
	FormatNanoWebM            MediaFormatName = "nanowebm"
	FormatTransparentWebP     MediaFormatName = "webp_transparent"
	FormatTinyTransparentWebP MediaFormatName = "tinywebp_transparent"
	FormatNanoTransparentWebP MediaFormatName = "nanowebp_transparent"
	FormatTransparentGIF      MediaFormatName = "gif_transparent"
	FormatTinyTransparentGIF  MediaFormatName = "tinygif_transparent"
	FormatNanoTransparentGIF  MediaFormatName = "nanogif_transparent"
)

// DiscoveryOptions contains parameters shared by Search and Featured.
type DiscoveryOptions struct {
	Country      string
	Locale       string
	Safety       SafetyFilter
	MediaFormats []MediaFormatName
	AspectRatio  AspectRatioRange
	Limit        int
	NextPosition string
}

// SearchRequest describes a Tenor Search request.
type SearchRequest struct {
	Query   string
	Content ContentKind
	Random  *bool
	DiscoveryOptions
}

// FeaturedRequest describes a Tenor Featured request.
type FeaturedRequest struct {
	Content ContentKind
	DiscoveryOptions
}

// CategoriesRequest describes a featured or trending category request.
type CategoriesRequest struct {
	Type    CategoryType
	Country string
	Locale  string
	Safety  SafetyFilter
}

// PostsRequest looks up at most 50 Tenor response-object IDs.
type PostsRequest struct {
	IDs          []string
	MediaFormats []MediaFormatName
}

// ResponseMeta exposes the cache directive callers must honor.
type ResponseMeta struct {
	CacheControl string
	ETag         string
	RequestID    string
}

// MediaObject is Tenor's complete documented media-object schema.
type MediaObject struct {
	URL      string  `json:"url"`
	Dims     []int   `json:"dims"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
}

// Post is the documented Tenor response object. MediaFormats remains a map so
// newly returned format names do not require guessing a fixed GIF schema.
type Post struct {
	Created            float64                         `json:"created"`
	HasAudio           bool                            `json:"hasaudio"`
	ID                 string                          `json:"id"`
	MediaFormats       map[MediaFormatName]MediaObject `json:"media_formats"`
	Tags               []string                        `json:"tags"`
	Title              string                          `json:"title"`
	ContentDescription string                          `json:"content_description"`
	ItemURL            string                          `json:"itemurl"`
	HasCaption         bool                            `json:"hascaption"`
	Flags              string                          `json:"flags"`
	BackgroundColor    string                          `json:"bg_color"`
	URL                string                          `json:"url"`
}

// Page is one Search or Featured page. Pass a non-empty NextPosition back as
// DiscoveryOptions.NextPosition; it is opaque and is not an array index.
type Page struct {
	Posts        []Post
	NextPosition string
	Meta         ResponseMeta
}

// Category is Tenor's documented category object. Path is returned without
// key or client_key query values and is never followed by this adapter.
type Category struct {
	SearchTerm string `json:"searchterm"`
	Path       string `json:"path"`
	Image      string `json:"image"`
	Name       string `json:"name"`
}

// CategoriesResponse contains localized category objects.
type CategoriesResponse struct {
	Categories []Category
	Meta       ResponseMeta
}

// PostsResponse contains objects returned by the Posts lookup endpoint.
type PostsResponse struct {
	Posts []Post
	Meta  ResponseMeta
}

type pageEnvelope struct {
	Next    string `json:"next"`
	Results []Post `json:"results"`
}

type categoriesEnvelope struct {
	Tags []Category `json:"tags"`
}

type postsEnvelope struct {
	Results []Post `json:"results"`
}

// DiscoveryWorkflow exposes the minimal high-value Tenor API v2 read surface.
type DiscoveryWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (Page, error)
	Featured(context.Context, FeaturedRequest, ...socialhub.CallOption) (Page, error)
	Categories(context.Context, CategoriesRequest, ...socialhub.CallOption) (CategoriesResponse, error)
	Posts(context.Context, PostsRequest, ...socialhub.CallOption) (PostsResponse, error)
}
