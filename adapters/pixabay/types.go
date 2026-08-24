package pixabay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MinimumPageSize        = 3
	MaximumPageSize        = 200
	DefaultPageSize        = 20
	RequiredCacheTTL       = 24 * time.Hour
	maxProviderObjectBytes = 8 << 20
)

type Language string

const (
	LanguageCS Language = "cs"
	LanguageDA Language = "da"
	LanguageDE Language = "de"
	LanguageEN Language = "en"
	LanguageES Language = "es"
	LanguageFR Language = "fr"
	LanguageID Language = "id"
	LanguageIT Language = "it"
	LanguageHU Language = "hu"
	LanguageNL Language = "nl"
	LanguageNO Language = "no"
	LanguagePL Language = "pl"
	LanguagePT Language = "pt"
	LanguageRO Language = "ro"
	LanguageSK Language = "sk"
	LanguageFI Language = "fi"
	LanguageSV Language = "sv"
	LanguageTR Language = "tr"
	LanguageVI Language = "vi"
	LanguageTH Language = "th"
	LanguageBG Language = "bg"
	LanguageRU Language = "ru"
	LanguageEL Language = "el"
	LanguageJA Language = "ja"
	LanguageKO Language = "ko"
	LanguageZH Language = "zh"
)

type Category string

const (
	CategoryBackgrounds    Category = "backgrounds"
	CategoryFashion        Category = "fashion"
	CategoryNature         Category = "nature"
	CategoryScience        Category = "science"
	CategoryEducation      Category = "education"
	CategoryFeelings       Category = "feelings"
	CategoryHealth         Category = "health"
	CategoryPeople         Category = "people"
	CategoryReligion       Category = "religion"
	CategoryPlaces         Category = "places"
	CategoryAnimals        Category = "animals"
	CategoryIndustry       Category = "industry"
	CategoryComputer       Category = "computer"
	CategoryFood           Category = "food"
	CategorySports         Category = "sports"
	CategoryTransportation Category = "transportation"
	CategoryTravel         Category = "travel"
	CategoryBuildings      Category = "buildings"
	CategoryBusiness       Category = "business"
	CategoryMusic          Category = "music"
)

type Order string

const (
	OrderPopular Order = "popular"
	OrderLatest  Order = "latest"
)

type ImageType string

const (
	ImageTypeAll          ImageType = "all"
	ImageTypePhoto        ImageType = "photo"
	ImageTypeIllustration ImageType = "illustration"
	ImageTypeVector       ImageType = "vector"
)

type Orientation string

const (
	OrientationAll        Orientation = "all"
	OrientationHorizontal Orientation = "horizontal"
	OrientationVertical   Orientation = "vertical"
)

type Color string

const (
	ColorGrayscale   Color = "grayscale"
	ColorTransparent Color = "transparent"
	ColorRed         Color = "red"
	ColorOrange      Color = "orange"
	ColorYellow      Color = "yellow"
	ColorGreen       Color = "green"
	ColorTurquoise   Color = "turquoise"
	ColorBlue        Color = "blue"
	ColorLilac       Color = "lilac"
	ColorPink        Color = "pink"
	ColorWhite       Color = "white"
	ColorGray        Color = "gray"
	ColorBlack       Color = "black"
	ColorBrown       Color = "brown"
)

type VideoType string

const (
	VideoTypeAll       VideoType = "all"
	VideoTypeFilm      VideoType = "film"
	VideoTypeAnimation VideoType = "animation"
)

type SearchRequest struct {
	Query         string
	Language      Language
	Category      Category
	MinimumWidth  int
	MinimumHeight int
	EditorsChoice bool
	SafeSearch    bool
	Order         Order
	Page          int
	PerPage       int
}

type ImageSearchRequest struct {
	SearchRequest
	ImageType   ImageType
	Orientation Orientation
	Colors      []Color
}

type VideoSearchRequest struct {
	SearchRequest
	VideoType VideoType
}

type CatalogWorkflow interface {
	SearchImages(context.Context, ImageSearchRequest, ...socialhub.CallOption) (ImageSearchResponse, error)
	SearchVideos(context.Context, VideoSearchRequest, ...socialhub.CallOption) (VideoSearchResponse, error)
}

// ResponseMeta preserves quota and cache information needed to comply with
// Pixabay's API policy. RateLimitReset is a remaining duration in seconds.
type ResponseMeta struct {
	RequestID           string
	CacheControl        string
	RateLimitLimit      string
	RateLimitRemaining  string
	RateLimitReset      string
	RateLimitResetAfter time.Duration
	RequiredCacheTTL    time.Duration
	Page                int
	PerPage             int
}

type Image struct {
	ID              int64           `json:"id"`
	PageURL         string          `json:"pageURL"`
	Type            ImageType       `json:"type"`
	Tags            string          `json:"tags"`
	PreviewURL      string          `json:"previewURL"`
	PreviewWidth    int64           `json:"previewWidth"`
	PreviewHeight   int64           `json:"previewHeight"`
	WebformatURL    string          `json:"webformatURL"`
	WebformatWidth  int64           `json:"webformatWidth"`
	WebformatHeight int64           `json:"webformatHeight"`
	LargeImageURL   string          `json:"largeImageURL"`
	FullHDURL       string          `json:"fullHDURL"`
	ImageURL        string          `json:"imageURL"`
	VectorURL       string          `json:"vectorURL"`
	ImageWidth      int64           `json:"imageWidth"`
	ImageHeight     int64           `json:"imageHeight"`
	ImageSize       int64           `json:"imageSize"`
	Views           int64           `json:"views"`
	Downloads       int64           `json:"downloads"`
	Likes           int64           `json:"likes"`
	Comments        int64           `json:"comments"`
	UserID          int64           `json:"user_id"`
	User            string          `json:"user"`
	UserImageURL    string          `json:"userImageURL"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Image) UnmarshalJSON(data []byte) error {
	type wire Image
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Image(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type VideoRendition struct {
	URL       string `json:"url"`
	Width     int64  `json:"width"`
	Height    int64  `json:"height"`
	Size      int64  `json:"size"`
	Thumbnail string `json:"thumbnail"`
}

type VideoRenditions struct {
	Large  VideoRendition `json:"large"`
	Medium VideoRendition `json:"medium"`
	Small  VideoRendition `json:"small"`
	Tiny   VideoRendition `json:"tiny"`
}

type Video struct {
	ID           int64           `json:"id"`
	PageURL      string          `json:"pageURL"`
	Type         VideoType       `json:"type"`
	Tags         string          `json:"tags"`
	Duration     int64           `json:"duration"`
	Videos       VideoRenditions `json:"videos"`
	Views        int64           `json:"views"`
	Downloads    int64           `json:"downloads"`
	Likes        int64           `json:"likes"`
	Comments     int64           `json:"comments"`
	UserID       int64           `json:"user_id"`
	User         string          `json:"user"`
	UserImageURL string          `json:"userImageURL"`
	Raw          json.RawMessage `json:"-"`
}

func (value *Video) UnmarshalJSON(data []byte) error {
	type wire Video
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Video(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ImageSearchResponse struct {
	Total     int64           `json:"total"`
	TotalHits int64           `json:"totalHits"`
	Hits      []Image         `json:"hits"`
	Meta      ResponseMeta    `json:"-"`
	Raw       json.RawMessage `json:"-"`
}

func (value *ImageSearchResponse) UnmarshalJSON(data []byte) error {
	type wire ImageSearchResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ImageSearchResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type VideoSearchResponse struct {
	Total     int64           `json:"total"`
	TotalHits int64           `json:"totalHits"`
	Hits      []Video         `json:"hits"`
	Meta      ResponseMeta    `json:"-"`
	Raw       json.RawMessage `json:"-"`
}

func (value *VideoSearchResponse) UnmarshalJSON(data []byte) error {
	type wire VideoSearchResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = VideoSearchResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("pixabay: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
