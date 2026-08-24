package googlephotos

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

// ResponseMeta preserves Google request, quota, caching, and lifecycle headers.
// Quota headers are optional; Google Cloud Console remains authoritative.
type ResponseMeta struct {
	RequestID          string
	QuotaProject       string
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
	RetryAfter         string
	ETag               string
	CacheControl       string
	ServerTiming       string
	Deprecation        string
	Sunset             string
	Warning            string
	RetryAfterDuration time.Duration
}

// PageOptions controls one list or search page.
type PageOptions struct {
	PageSize  int
	PageToken string
}

// Page is one provider page with its opaque continuation and bounded raw JSON.
type Page[T any] struct {
	Items         []T
	NextPageToken string
	Meta          ResponseMeta
	Raw           json.RawMessage
}

// Album is an app-created Google Photos album.
type Album struct {
	ID                    string          `json:"id"`
	Title                 string          `json:"title"`
	ProductURL            string          `json:"productUrl"`
	IsWriteable           bool            `json:"isWriteable"`
	MediaItemsCount       uint64          `json:"mediaItemsCount,string"`
	CoverPhotoBaseURL     string          `json:"coverPhotoBaseUrl"`
	CoverPhotoMediaItemID string          `json:"coverPhotoMediaItemId"`
	Raw                   json.RawMessage `json:"-"`
}

func (value *Album) UnmarshalJSON(data []byte) error {
	type wire Album
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Album(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// Photo contains photo-specific capture metadata.
type Photo struct {
	CameraMake      string  `json:"cameraMake"`
	CameraModel     string  `json:"cameraModel"`
	FocalLength     float64 `json:"focalLength"`
	ApertureFNumber float64 `json:"apertureFNumber"`
	ISOEquivalent   int32   `json:"isoEquivalent"`
	ExposureTime    string  `json:"exposureTime"`
}

// VideoStatus is the current Google Photos video processing state.
type VideoStatus string

const (
	VideoStatusUnspecified VideoStatus = "UNSPECIFIED"
	VideoStatusProcessing  VideoStatus = "PROCESSING"
	VideoStatusReady       VideoStatus = "READY"
	VideoStatusFailed      VideoStatus = "FAILED"
)

// Video contains video-specific capture and processing metadata.
type Video struct {
	CameraMake  string      `json:"cameraMake"`
	CameraModel string      `json:"cameraModel"`
	FPS         float64     `json:"fps"`
	Status      VideoStatus `json:"status"`
}

// MediaMetadata contains dimensions, capture time, and exactly one media kind
// when Google supplies type-specific metadata.
type MediaMetadata struct {
	CreationTime *time.Time `json:"creationTime"`
	Width        uint64     `json:"width,string"`
	Height       uint64     `json:"height,string"`
	Photo        *Photo     `json:"photo"`
	Video        *Video     `json:"video"`
}

// MediaItem is an app-created photo or video. BaseURL is short lived and must
// be parameterized according to Google's media-byte documentation before use.
type MediaItem struct {
	ID            string          `json:"id"`
	Description   string          `json:"description"`
	ProductURL    string          `json:"productUrl"`
	BaseURL       string          `json:"baseUrl"`
	MIMEType      string          `json:"mimeType"`
	Filename      string          `json:"filename"`
	MediaMetadata *MediaMetadata  `json:"mediaMetadata"`
	Raw           json.RawMessage `json:"-"`
}

func (value *MediaItem) UnmarshalJSON(data []byte) error {
	type wire MediaItem
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = MediaItem(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// Date is Google Photos' partial calendar date. Zero components have the
// provider meanings documented for birthday, month, and year searches.
type Date struct {
	Year  int `json:"year,omitempty"`
	Month int `json:"month,omitempty"`
	Day   int `json:"day,omitempty"`
}

// DateRange is an inclusive range whose endpoints use the same partial format.
type DateRange struct {
	StartDate Date `json:"startDate"`
	EndDate   Date `json:"endDate"`
}

// DateFilter accepts up to five individual dates and five inclusive ranges.
type DateFilter struct {
	Dates  []Date      `json:"dates,omitempty"`
	Ranges []DateRange `json:"ranges,omitempty"`
}

// MediaType is a Google Photos search media category.
type MediaType string

const (
	MediaTypeAll   MediaType = "ALL_MEDIA"
	MediaTypeVideo MediaType = "VIDEO"
	MediaTypePhoto MediaType = "PHOTO"
)

// MediaTypeFilter follows the provider shape; at most one value is valid.
type MediaTypeFilter struct {
	MediaTypes []MediaType `json:"mediaTypes,omitempty"`
}

// ContentCategory is a provider content classifier used by search.
type ContentCategory string

const (
	ContentNone         ContentCategory = "NONE"
	ContentLandscapes   ContentCategory = "LANDSCAPES"
	ContentReceipts     ContentCategory = "RECEIPTS"
	ContentCityscapes   ContentCategory = "CITYSCAPES"
	ContentLandmarks    ContentCategory = "LANDMARKS"
	ContentSelfies      ContentCategory = "SELFIES"
	ContentPeople       ContentCategory = "PEOPLE"
	ContentPets         ContentCategory = "PETS"
	ContentWeddings     ContentCategory = "WEDDINGS"
	ContentBirthdays    ContentCategory = "BIRTHDAYS"
	ContentDocuments    ContentCategory = "DOCUMENTS"
	ContentTravel       ContentCategory = "TRAVEL"
	ContentAnimals      ContentCategory = "ANIMALS"
	ContentFood         ContentCategory = "FOOD"
	ContentSport        ContentCategory = "SPORT"
	ContentNight        ContentCategory = "NIGHT"
	ContentPerformances ContentCategory = "PERFORMANCES"
	ContentWhiteboards  ContentCategory = "WHITEBOARDS"
	ContentScreenshots  ContentCategory = "SCREENSHOTS"
	ContentUtility      ContentCategory = "UTILITY"
	ContentArts         ContentCategory = "ARTS"
	ContentCrafts       ContentCategory = "CRAFTS"
	ContentFashion      ContentCategory = "FASHION"
	ContentHouses       ContentCategory = "HOUSES"
	ContentGardens      ContentCategory = "GARDENS"
	ContentFlowers      ContentCategory = "FLOWERS"
	ContentHolidays     ContentCategory = "HOLIDAYS"
)

// ContentFilter includes and excludes OR-ed categories. A category cannot
// appear in both sets.
type ContentFilter struct {
	IncludedContentCategories []ContentCategory `json:"includedContentCategories,omitempty"`
	ExcludedContentCategories []ContentCategory `json:"excludedContentCategories,omitempty"`
}

// Feature is a Google Photos search feature.
type Feature string

const (
	FeatureNone      Feature = "NONE"
	FeatureFavorites Feature = "FAVORITES"
)

type FeatureFilter struct {
	IncludedFeatures []Feature `json:"includedFeatures,omitempty"`
}

// SearchFilters is the current app-created-media search filter shape. The
// removed excludeNonAppCreatedData switch is intentionally not exposed.
type SearchFilters struct {
	IncludeArchivedMedia bool             `json:"includeArchivedMedia,omitempty"`
	DateFilter           *DateFilter      `json:"dateFilter,omitempty"`
	ContentFilter        *ContentFilter   `json:"contentFilter,omitempty"`
	FeatureFilter        *FeatureFilter   `json:"featureFilter,omitempty"`
	MediaTypeFilter      *MediaTypeFilter `json:"mediaTypeFilter,omitempty"`
}

// SearchOrder is valid only with a date filter.
type SearchOrder string

const (
	SearchOrderOldestFirst SearchOrder = "MediaMetadata.creation_time"
	SearchOrderNewestFirst SearchOrder = "MediaMetadata.creation_time desc"
)

type ListAlbumsRequest struct {
	Page PageOptions
}

type ListMediaItemsRequest struct {
	Page PageOptions
}

type SearchMediaItemsRequest struct {
	AlbumID string
	Filters *SearchFilters
	OrderBy SearchOrder
	Page    PageOptions
}

// ReadWorkflow is the complete app-created-data surface implemented here.
type ReadWorkflow interface {
	ListAlbums(context.Context, ListAlbumsRequest, ...socialhub.CallOption) (*Page[Album], error)
	GetAlbum(context.Context, string, ...socialhub.CallOption) (*Album, ResponseMeta, error)
	ListMediaItems(context.Context, ListMediaItemsRequest, ...socialhub.CallOption) (*Page[MediaItem], error)
	GetMediaItem(context.Context, string, ...socialhub.CallOption) (*MediaItem, ResponseMeta, error)
	SearchMediaItems(context.Context, SearchMediaItemsRequest, ...socialhub.CallOption) (*Page[MediaItem], error)
}
