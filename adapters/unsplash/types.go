package unsplash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"social-hub/pkg/socialhub"
)

const (
	MaximumPageSize        = 30
	maxProviderObjectBytes = 8 << 20
)

type SearchOrder string

const (
	SearchOrderLatest    SearchOrder = "latest"
	SearchOrderEditorial SearchOrder = "editorial"
	SearchOrderRelevant  SearchOrder = "relevant"
)

type UserPhotoOrder string

const (
	UserPhotoOrderLatest    UserPhotoOrder = "latest"
	UserPhotoOrderOldest    UserPhotoOrder = "oldest"
	UserPhotoOrderPopular   UserPhotoOrder = "popular"
	UserPhotoOrderViews     UserPhotoOrder = "views"
	UserPhotoOrderDownloads UserPhotoOrder = "downloads"
	UserPhotoOrderPinned    UserPhotoOrder = "pinned"
)

type Orientation string

const (
	OrientationLandscape Orientation = "landscape"
	OrientationPortrait  Orientation = "portrait"
	OrientationSquarish  Orientation = "squarish"
)

type ContentFilter string

const (
	ContentFilterLow  ContentFilter = "low"
	ContentFilterHigh ContentFilter = "high"
)

// SearchLanguage selects one of Unsplash's documented search languages.
type SearchLanguage string

const (
	SearchLanguageEnglish            SearchLanguage = "en"
	SearchLanguageSimplifiedChinese  SearchLanguage = "zh-Hans"
	SearchLanguageTraditionalChinese SearchLanguage = "zh-Hant"
)

type Color string

const (
	ColorBlackAndWhite Color = "black_and_white"
	ColorBlack         Color = "black"
	ColorWhite         Color = "white"
	ColorYellow        Color = "yellow"
	ColorOrange        Color = "orange"
	ColorRed           Color = "red"
	ColorPurple        Color = "purple"
	ColorMagenta       Color = "magenta"
	ColorGreen         Color = "green"
	ColorTeal          Color = "teal"
	ColorBlue          Color = "blue"
)

type PageRequest struct {
	Page    int
	PerPage int
}

type SearchPhotosRequest struct {
	Query         string
	Page          int
	PerPage       int
	OrderBy       SearchOrder
	Collections   []string
	ContentFilter ContentFilter
	Color         Color
	Orientation   Orientation
	Language      SearchLanguage
}

type ListUserPhotosRequest struct {
	Username    string
	Page        int
	PerPage     int
	OrderBy     UserPhotoOrder
	Orientation Orientation
}

type ListUserCollectionsRequest struct {
	Username string
	Page     int
	PerPage  int
}

type ListCollectionPhotosRequest struct {
	CollectionID string
	Page         int
	PerPage      int
	Orientation  Orientation
}

type PhotosWorkflow interface {
	SearchPhotos(context.Context, SearchPhotosRequest, ...socialhub.CallOption) (SearchPhotosResponse, error)
	GetPhoto(context.Context, string, ...socialhub.CallOption) (Photo, error)
	TrackDownload(context.Context, string, ...socialhub.CallOption) (ResponseMeta, error)
}

type UsersWorkflow interface {
	GetUser(context.Context, string, ...socialhub.CallOption) (User, error)
	ListUserPhotos(context.Context, ListUserPhotosRequest, ...socialhub.CallOption) (PhotoPage, error)
	ListUserCollections(context.Context, ListUserCollectionsRequest, ...socialhub.CallOption) (CollectionPage, error)
}

type CollectionsWorkflow interface {
	ListCollections(context.Context, PageRequest, ...socialhub.CallOption) (CollectionPage, error)
	GetCollection(context.Context, string, ...socialhub.CallOption) (Collection, error)
	ListCollectionPhotos(context.Context, ListCollectionPhotosRequest, ...socialhub.CallOption) (PhotoPage, error)
}

// ResponseMeta preserves documented quota and pagination headers. Header
// values remain text so provider changes are not silently coerced.
type ResponseMeta struct {
	StatusCode         int
	RequestID          string
	RateLimitLimit     string
	RateLimitRemaining string
	Total              string
	Link               string
	Warning            string
	Page               int
	PerPage            int
	NextPage           *int
	PreviousPage       *int
}

// Identifier accepts the current string collection ID and the numeric IDs
// still present in the official endpoint examples.
type Identifier string

func (value *Identifier) UnmarshalJSON(data []byte) error {
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		number := json.Number(string(data))
		parsed, err := strconv.ParseUint(number.String(), 10, 64)
		if err != nil {
			return fmt.Errorf("unsplash: identifier must be a string or non-negative integer")
		}
		text = strconv.FormatUint(parsed, 10)
	}
	if !validResourceID(text) {
		return fmt.Errorf("unsplash: identifier is invalid")
	}
	*value = Identifier(text)
	return nil
}

type PhotoURLs struct {
	Raw     string `json:"raw"`
	Full    string `json:"full"`
	Regular string `json:"regular"`
	Small   string `json:"small"`
	Thumb   string `json:"thumb"`
}

type PhotoLinks struct {
	Self             string `json:"self"`
	HTML             string `json:"html"`
	Download         string `json:"download"`
	DownloadLocation string `json:"download_location"`
}

type ProfileImage struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

type UserLinks struct {
	Self      string `json:"self"`
	HTML      string `json:"html"`
	Photos    string `json:"photos"`
	Likes     string `json:"likes"`
	Portfolio string `json:"portfolio"`
	Following string `json:"following"`
	Followers string `json:"followers"`
}

type UserSocial struct {
	InstagramUsername *string `json:"instagram_username"`
	PortfolioURL      *string `json:"portfolio_url"`
	TwitterUsername   *string `json:"twitter_username"`
	PayPalEmail       *string `json:"paypal_email"`
}

type Badge struct {
	Title   string  `json:"title"`
	Primary bool    `json:"primary"`
	Slug    string  `json:"slug"`
	Link    *string `json:"link"`
}

type User struct {
	ID                string          `json:"id"`
	UpdatedAt         string          `json:"updated_at"`
	Username          string          `json:"username"`
	Name              string          `json:"name"`
	FirstName         string          `json:"first_name"`
	LastName          *string         `json:"last_name"`
	Bio               *string         `json:"bio"`
	Location          *string         `json:"location"`
	PortfolioURL      *string         `json:"portfolio_url"`
	TwitterUsername   *string         `json:"twitter_username"`
	InstagramUsername *string         `json:"instagram_username"`
	TotalCollections  int             `json:"total_collections"`
	TotalLikes        int             `json:"total_likes"`
	TotalPhotos       int             `json:"total_photos"`
	Downloads         int             `json:"downloads"`
	AcceptedTOS       bool            `json:"accepted_tos"`
	ForHire           bool            `json:"for_hire"`
	Links             UserLinks       `json:"links"`
	ProfileImage      ProfileImage    `json:"profile_image"`
	Social            UserSocial      `json:"social"`
	Badge             *Badge          `json:"badge"`
	Meta              ResponseMeta    `json:"-"`
	Raw               json.RawMessage `json:"-"`
}

func (value *User) UnmarshalJSON(data []byte) error {
	type wire User
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = User(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Position struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type PhotoLocation struct {
	Name     *string  `json:"name"`
	City     *string  `json:"city"`
	Country  *string  `json:"country"`
	Position Position `json:"position"`
}

type Exif struct {
	Name         *string  `json:"name"`
	Make         *string  `json:"make"`
	Model        *string  `json:"model"`
	ExposureTime *string  `json:"exposure_time"`
	Aperture     *string  `json:"aperture"`
	FocalLength  *string  `json:"focal_length"`
	ISO          *float64 `json:"iso"`
}

type Tag struct {
	Type  string `json:"type"`
	Title string `json:"title"`
}

type CurrentUserCollection struct {
	ID              Identifier      `json:"id"`
	Title           string          `json:"title"`
	Description     *string         `json:"description"`
	PublishedAt     string          `json:"published_at"`
	LastCollectedAt *string         `json:"last_collected_at"`
	UpdatedAt       string          `json:"updated_at"`
	Featured        bool            `json:"featured"`
	TotalPhotos     int             `json:"total_photos"`
	Private         bool            `json:"private"`
	ShareKey        string          `json:"share_key"`
	Links           CollectionLinks `json:"links"`
}

type Photo struct {
	ID                     string                  `json:"id"`
	Slug                   string                  `json:"slug"`
	CreatedAt              string                  `json:"created_at"`
	UpdatedAt              string                  `json:"updated_at"`
	PromotedAt             *string                 `json:"promoted_at"`
	Width                  int                     `json:"width"`
	Height                 int                     `json:"height"`
	Color                  *string                 `json:"color"`
	BlurHash               *string                 `json:"blur_hash"`
	Description            *string                 `json:"description"`
	AltDescription         *string                 `json:"alt_description"`
	Likes                  int                     `json:"likes"`
	LikedByUser            bool                    `json:"liked_by_user"`
	Downloads              int                     `json:"downloads"`
	PublicDomain           bool                    `json:"public_domain"`
	URLs                   PhotoURLs               `json:"urls"`
	Links                  PhotoLinks              `json:"links"`
	User                   *User                   `json:"user"`
	CurrentUserCollections []CurrentUserCollection `json:"current_user_collections"`
	Exif                   *Exif                   `json:"exif"`
	Location               *PhotoLocation          `json:"location"`
	Tags                   []Tag                   `json:"tags"`
	Meta                   ResponseMeta            `json:"-"`
	Raw                    json.RawMessage         `json:"-"`
}

func (value *Photo) UnmarshalJSON(data []byte) error {
	type wire Photo
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Photo(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CollectionLinks struct {
	Self    string `json:"self"`
	HTML    string `json:"html"`
	Photos  string `json:"photos"`
	Related string `json:"related"`
}

type PreviewPhoto struct {
	ID        string    `json:"id"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	BlurHash  *string   `json:"blur_hash"`
	URLs      PhotoURLs `json:"urls"`
}

type Collection struct {
	ID              Identifier      `json:"id"`
	Title           string          `json:"title"`
	Description     *string         `json:"description"`
	PublishedAt     string          `json:"published_at"`
	LastCollectedAt *string         `json:"last_collected_at"`
	UpdatedAt       string          `json:"updated_at"`
	Featured        bool            `json:"featured"`
	TotalPhotos     int             `json:"total_photos"`
	Private         bool            `json:"private"`
	ShareKey        string          `json:"share_key"`
	Links           CollectionLinks `json:"links"`
	User            *User           `json:"user"`
	CoverPhoto      *Photo          `json:"cover_photo"`
	PreviewPhotos   []PreviewPhoto  `json:"preview_photos"`
	Meta            ResponseMeta    `json:"-"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Collection) UnmarshalJSON(data []byte) error {
	type wire Collection
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Collection(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type SearchPhotosResponse struct {
	Total      int             `json:"total"`
	TotalPages int             `json:"total_pages"`
	Results    []Photo         `json:"results"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *SearchPhotosResponse) UnmarshalJSON(data []byte) error {
	type wire SearchPhotosResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchPhotosResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PhotoPage struct {
	Photos []Photo
	Meta   ResponseMeta
	Raw    json.RawMessage
}

type CollectionPage struct {
	Collections []Collection
	Meta        ResponseMeta
	Raw         json.RawMessage
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("unsplash: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

func decodeProviderArray(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '[' || !json.Valid(trimmed) {
		return fmt.Errorf("unsplash: invalid provider array")
	}
	return json.Unmarshal(trimmed, target)
}
