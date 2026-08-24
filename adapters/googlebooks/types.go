package googlebooks

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	DefaultMaxResults = 10
	MaximumMaxResults = 40
)

type VolumeFilter string

const (
	VolumeFilterEBooks     VolumeFilter = "ebooks"
	VolumeFilterFreeEBooks VolumeFilter = "free-ebooks"
	VolumeFilterFull       VolumeFilter = "full"
	VolumeFilterPaidEBooks VolumeFilter = "paid-ebooks"
	VolumeFilterPartial    VolumeFilter = "partial"
)

type VolumeOrder string

const (
	VolumeOrderRelevance VolumeOrder = "relevance"
	VolumeOrderNewest    VolumeOrder = "newest"
)

type SearchPrintType string

const (
	SearchPrintAll       SearchPrintType = "all"
	SearchPrintBooks     SearchPrintType = "books"
	SearchPrintMagazines SearchPrintType = "magazines"
)

type Projection string

const (
	ProjectionFull Projection = "full"
	ProjectionLite Projection = "lite"
)

type SearchVolumesRequest struct {
	Query      string
	StartIndex int
	MaxResults int
	Filter     VolumeFilter
	OrderBy    VolumeOrder
	PrintType  SearchPrintType
	Projection Projection
	Language   string
}

type GetVolumeRequest struct {
	VolumeID   string
	Projection Projection
}

type IndustryIdentifierType string

const (
	IdentifierISBN10 IndustryIdentifierType = "ISBN_10"
	IdentifierISBN13 IndustryIdentifierType = "ISBN_13"
	IdentifierISSN   IndustryIdentifierType = "ISSN"
	IdentifierOther  IndustryIdentifierType = "OTHER"
)

type IndustryIdentifier struct {
	Type       IndustryIdentifierType `json:"type"`
	Identifier string                 `json:"identifier"`
}

type ReadingModes struct {
	Text  bool `json:"text"`
	Image bool `json:"image"`
}

type Dimensions struct {
	Height    string `json:"height"`
	Width     string `json:"width"`
	Thickness string `json:"thickness"`
}

type ImageLinks struct {
	SmallThumbnail string `json:"smallThumbnail"`
	Thumbnail      string `json:"thumbnail"`
	Small          string `json:"small"`
	Medium         string `json:"medium"`
	Large          string `json:"large"`
	ExtraLarge     string `json:"extraLarge"`
}

type PublicationType string

const (
	PublicationBook     PublicationType = "BOOK"
	PublicationMagazine PublicationType = "MAGAZINE"
)

// VolumeInfo contains public bibliographic and descriptive metadata. The
// description may contain simple HTML supplied by Google or a publisher.
type VolumeInfo struct {
	Title               string               `json:"title"`
	Subtitle            string               `json:"subtitle"`
	Authors             []string             `json:"authors"`
	Publisher           string               `json:"publisher"`
	PublishedDate       string               `json:"publishedDate"`
	Description         string               `json:"description"`
	IndustryIdentifiers []IndustryIdentifier `json:"industryIdentifiers"`
	ReadingModes        ReadingModes         `json:"readingModes"`
	PageCount           int                  `json:"pageCount"`
	PrintedPageCount    int                  `json:"printedPageCount"`
	Dimensions          Dimensions           `json:"dimensions"`
	PrintType           PublicationType      `json:"printType"`
	MainCategory        string               `json:"mainCategory"`
	Categories          []string             `json:"categories"`
	AverageRating       float64              `json:"averageRating"`
	RatingsCount        int                  `json:"ratingsCount"`
	ContentVersion      string               `json:"contentVersion"`
	ImageLinks          ImageLinks           `json:"imageLinks"`
	Language            string               `json:"language"`
	PreviewLink         string               `json:"previewLink"`
	InfoLink            string               `json:"infoLink"`
	CanonicalVolumeLink string               `json:"canonicalVolumeLink"`
	MaturityRating      string               `json:"maturityRating"`
}

func (VolumeInfo) String() string   { return "googlebooks.VolumeInfo{REDACTED}" }
func (VolumeInfo) GoString() string { return "googlebooks.VolumeInfo{REDACTED}" }

type Viewability string

const (
	ViewabilityPartial  Viewability = "PARTIAL"
	ViewabilityAllPages Viewability = "ALL_PAGES"
	ViewabilityNoPages  Viewability = "NO_PAGES"
	ViewabilityUnknown  Viewability = "UNKNOWN"
)

type TextToSpeechPermission string

const (
	TextToSpeechAllowed              TextToSpeechPermission = "ALLOWED"
	TextToSpeechAllowedAccessibility TextToSpeechPermission = "ALLOWED_FOR_ACCESSIBILITY"
	TextToSpeechNotAllowed           TextToSpeechPermission = "NOT_ALLOWED"
)

// AccessInfo excludes purchase, order, download-token, and user-library data.
type AccessInfo struct {
	Country                string                 `json:"country"`
	Viewability            Viewability            `json:"viewability"`
	Embeddable             bool                   `json:"embeddable"`
	PublicDomain           bool                   `json:"publicDomain"`
	TextToSpeechPermission TextToSpeechPermission `json:"textToSpeechPermission"`
	WebReaderLink          string                 `json:"webReaderLink"`
}

type SearchInfo struct {
	TextSnippet string `json:"textSnippet"`
}

func (SearchInfo) String() string   { return "googlebooks.SearchInfo{REDACTED}" }
func (SearchInfo) GoString() string { return "googlebooks.SearchInfo{REDACTED}" }

// Volume deliberately omits saleInfo, userInfo, purchase links, and Raw JSON.
type Volume struct {
	Kind       string     `json:"kind"`
	ID         string     `json:"id"`
	ETag       string     `json:"etag"`
	SelfLink   string     `json:"selfLink"`
	Info       VolumeInfo `json:"volumeInfo"`
	Access     AccessInfo `json:"accessInfo"`
	SearchInfo SearchInfo `json:"searchInfo"`
}

func (Volume) String() string   { return "googlebooks.Volume{REDACTED}" }
func (Volume) GoString() string { return "googlebooks.Volume{REDACTED}" }

type ResponseMeta struct {
	RequestID    string
	RetryAfter   time.Duration
	QuotaHeaders map[string]string
}

type VolumePage struct {
	Items      []Volume
	TotalItems int64
	StartIndex int
	MaxResults int
	Meta       ResponseMeta
}

func (VolumePage) String() string   { return "googlebooks.VolumePage{REDACTED}" }
func (VolumePage) GoString() string { return "googlebooks.VolumePage{REDACTED}" }

type VolumeResult struct {
	Volume Volume
	Meta   ResponseMeta
}

func (VolumeResult) String() string   { return "googlebooks.VolumeResult{REDACTED}" }
func (VolumeResult) GoString() string { return "googlebooks.VolumeResult{REDACTED}" }

type VolumesWorkflow interface {
	Search(context.Context, SearchVolumesRequest, ...socialhub.CallOption) (*VolumePage, error)
	Get(context.Context, GetVolumeRequest, ...socialhub.CallOption) (*VolumeResult, error)
}

var _ VolumesWorkflow = (*Client)(nil)
