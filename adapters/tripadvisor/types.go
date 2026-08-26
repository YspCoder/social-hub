package tripadvisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumPageSize        = 5
	MaximumOffset          = 1<<31 - 1
	MaximumSearchResults   = 10
	maxProviderObjectBytes = 8 << 20
)

// ID is a canonical positive decimal Tripadvisor identifier. It accepts both
// JSON numbers and quoted decimal strings because provider payloads in the
// ecosystem have used both representations.
type ID string

func (value *ID) UnmarshalJSON(data []byte) error {
	var raw string
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return err
		}
	} else {
		raw = string(trimmed)
	}
	normalized, ok := normalizeDecimalID(raw)
	if !ok {
		return fmt.Errorf("tripadvisor: identifier must be a positive decimal integer")
	}
	*value = ID(normalized)
	return nil
}

func (value ID) String() string { return string(value) }

type Category string

const (
	CategoryHotels      Category = "hotels"
	CategoryAttractions Category = "attractions"
	CategoryRestaurants Category = "restaurants"
	CategoryGeos        Category = "geos"
)

type RadiusUnit string

const (
	RadiusKilometers RadiusUnit = "km"
	RadiusMiles      RadiusUnit = "mi"
	RadiusMeters     RadiusUnit = "m"
)

type PhotoSource string

const (
	PhotoSourceExpert     PhotoSource = "Expert"
	PhotoSourceManagement PhotoSource = "Management"
	PhotoSourceTraveler   PhotoSource = "Traveler"
)

type Coordinate struct {
	Latitude  float64
	Longitude float64
}

type SearchLocationsRequest struct {
	SearchQuery string
	Category    Category
	Phone       string
	Address     string
	Coordinate  *Coordinate
	Radius      *float64
	RadiusUnit  RadiusUnit
	Language    string
}

type SearchNearbyRequest struct {
	Coordinate Coordinate
	Category   Category
	Phone      string
	Address    string
	Radius     *float64
	RadiusUnit RadiusUnit
	Language   string
}

type GetLocationDetailsRequest struct {
	LocationID ID
	Language   string
	Currency   string
}

type ListPhotosRequest struct {
	LocationID ID
	Language   string
	Limit      int
	Offset     int
	Sources    []PhotoSource
}

type ListReviewsRequest struct {
	LocationID ID
	Language   string
	Limit      int
	Offset     int
}

// PlacesWorkflow exposes the five bounded Content API v1 read operations.
type PlacesWorkflow interface {
	SearchLocations(context.Context, SearchLocationsRequest, ...socialhub.CallOption) (SearchLocationsResponse, error)
	SearchNearby(context.Context, SearchNearbyRequest, ...socialhub.CallOption) (SearchLocationsResponse, error)
	GetLocationDetails(context.Context, GetLocationDetailsRequest, ...socialhub.CallOption) (LocationDetails, error)
	ListPhotos(context.Context, ListPhotosRequest, ...socialhub.CallOption) (PhotoPage, error)
	ListReviews(context.Context, ListReviewsRequest, ...socialhub.CallOption) (ReviewPage, error)
}

// ResponseMeta preserves useful API Gateway and rate-control headers.
type ResponseMeta struct {
	RequestID          string
	APIGatewayID       string
	RetryAfterHeader   string
	RetryAfter         time.Duration
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
}

type Address struct {
	Street1       string `json:"street1"`
	Street2       string `json:"street2"`
	City          string `json:"city"`
	State         string `json:"state"`
	Country       string `json:"country"`
	PostalCode    string `json:"postalcode"`
	AddressString string `json:"address_string"`
}

type SearchLocation struct {
	LocationID ID              `json:"location_id"`
	Name       string          `json:"name"`
	Distance   string          `json:"distance"`
	Bearing    string          `json:"bearing"`
	Address    Address         `json:"address_obj"`
	Raw        json.RawMessage `json:"-"`
}

func (value *SearchLocation) UnmarshalJSON(data []byte) error {
	type wire SearchLocation
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchLocation(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type SearchLocationsResponse struct {
	Data []SearchLocation `json:"data"`
	Meta ResponseMeta     `json:"-"`
	Raw  json.RawMessage  `json:"-"`
}

func (value *SearchLocationsResponse) UnmarshalJSON(data []byte) error {
	type wire SearchLocationsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchLocationsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Ancestor struct {
	Abbreviation string `json:"abbrv"`
	Level        string `json:"level"`
	Name         string `json:"name"`
	LocationID   ID     `json:"location_id"`
}

type RankingData struct {
	GeoLocationID   ID     `json:"geo_location_id"`
	RankingString   string `json:"ranking_string"`
	GeoLocationName string `json:"geo_location_name"`
	RankingOutOf    int    `json:"ranking_out_of"`
	Ranking         int    `json:"ranking"`
}

type NamedValue struct {
	Name          string `json:"name"`
	LocalizedName string `json:"localized_name"`
}

type Subrating struct {
	NamedValue
	RatingImageURL string  `json:"rating_image_url"`
	Value          float64 `json:"value"`
}

type DayTime struct {
	Day  int    `json:"day"`
	Time string `json:"time"`
}

type HoursPeriod struct {
	Open  DayTime `json:"open"`
	Close DayTime `json:"close"`
}

type Hours struct {
	Periods     []HoursPeriod `json:"periods"`
	WeekdayText []string      `json:"weekday_text"`
}

type Group struct {
	NamedValue
	Categories []NamedValue `json:"categories"`
}

type Neighborhood struct {
	LocationID ID     `json:"location_id"`
	Name       string `json:"name"`
}

type TripType struct {
	NamedValue
	Value string `json:"value"`
}

type AwardImages struct {
	Tiny  string `json:"tiny"`
	Small string `json:"small"`
	Large string `json:"large"`
}

type Award struct {
	AwardType   string      `json:"award_type"`
	Year        int         `json:"year"`
	Images      AwardImages `json:"images"`
	Categories  []string    `json:"categories"`
	DisplayName string      `json:"display_name"`
}

type LocationDetails struct {
	LocationID        ID                   `json:"location_id"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	WebURL            string               `json:"web_url"`
	Address           Address              `json:"address_obj"`
	Ancestors         []Ancestor           `json:"ancestors"`
	Latitude          *float64             `json:"latitude"`
	Longitude         *float64             `json:"longitude"`
	Timezone          string               `json:"timezone"`
	Email             string               `json:"email"`
	Phone             string               `json:"phone"`
	Website           string               `json:"website"`
	WriteReviewURL    string               `json:"write_review"`
	RankingData       *RankingData         `json:"ranking_data"`
	Rating            *float64             `json:"rating"`
	RatingImageURL    string               `json:"rating_image_url"`
	NumReviews        string               `json:"num_reviews"`
	ReviewRatingCount map[string]string    `json:"review_rating_count"`
	Subratings        map[string]Subrating `json:"subratings"`
	PhotoCount        *int                 `json:"photo_count"`
	SeeAllPhotosURL   string               `json:"see_all_photos"`
	PriceLevel        string               `json:"price_level"`
	Hours             *Hours               `json:"hours"`
	Amenities         []string             `json:"amenities"`
	Features          []string             `json:"features"`
	Cuisine           []NamedValue         `json:"cuisine"`
	ParentBrand       string               `json:"parent_brand"`
	Brand             string               `json:"brand"`
	Category          *NamedValue          `json:"category"`
	Subcategory       []NamedValue         `json:"subcategory"`
	Groups            []Group              `json:"groups"`
	Styles            []string             `json:"styles"`
	Neighborhoods     []Neighborhood       `json:"neighborhood_info"`
	TripTypes         []TripType           `json:"trip_types"`
	Awards            []Award              `json:"awards"`
	Meta              ResponseMeta         `json:"-"`
	Raw               json.RawMessage      `json:"-"`
}

func (value *LocationDetails) UnmarshalJSON(data []byte) error {
	type wire LocationDetails
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = LocationDetails(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type UserLocation struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type ContentUser struct {
	Username      string            `json:"username"`
	Location      *UserLocation     `json:"user_location"`
	ReviewCount   *int              `json:"review_count"`
	ReviewerBadge string            `json:"reviewer_badge"`
	Avatar        map[string]string `json:"avatar"`
}

type PhotoImage struct {
	Width  float64 `json:"width"`
	URL    string  `json:"url"`
	Height float64 `json:"height"`
}

type Photo struct {
	ID            ID                    `json:"id"`
	IsBlessed     *bool                 `json:"is_blessed"`
	Album         string                `json:"album"`
	Caption       string                `json:"caption"`
	PublishedDate string                `json:"published_date"`
	Images        map[string]PhotoImage `json:"images"`
	Source        NamedValue            `json:"source"`
	User          *ContentUser          `json:"user"`
	Raw           json.RawMessage       `json:"-"`
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

type Paging struct {
	Next         string `json:"next"`
	Previous     string `json:"previous"`
	Results      int    `json:"results"`
	TotalResults int    `json:"total_results"`
	Skipped      int    `json:"skipped"`
}

type PhotoPage struct {
	Data       []Photo         `json:"data"`
	Paging     Paging          `json:"paging"`
	LocationID ID              `json:"-"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *PhotoPage) UnmarshalJSON(data []byte) error {
	type wire PhotoPage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PhotoPage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type OwnerResponse struct {
	ID            ID     `json:"id"`
	Language      string `json:"lang"`
	Text          string `json:"text"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	PublishedDate string `json:"published_date"`
}

type Review struct {
	ID                  ID                   `json:"id"`
	Language            string               `json:"lang"`
	LocationID          ID                   `json:"location_id"`
	PublishedDate       string               `json:"published_date"`
	Rating              *int                 `json:"rating"`
	HelpfulVotes        *int                 `json:"helpful_votes"`
	RatingImageURL      string               `json:"rating_image_url"`
	URL                 string               `json:"url"`
	TripType            string               `json:"trip_type"`
	TravelDate          string               `json:"travel_date"`
	Text                string               `json:"text"`
	Title               string               `json:"title"`
	OwnerResponse       *OwnerResponse       `json:"owner_response"`
	IsMachineTranslated *bool                `json:"is_machine_translated"`
	User                *ContentUser         `json:"user"`
	Subratings          map[string]Subrating `json:"subratings"`
	Raw                 json.RawMessage      `json:"-"`
}

func (value *Review) UnmarshalJSON(data []byte) error {
	type wire Review
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Review(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ReviewPage struct {
	Data       []Review        `json:"data"`
	Paging     Paging          `json:"paging"`
	LocationID ID              `json:"-"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *ReviewPage) UnmarshalJSON(data []byte) error {
	type wire ReviewPage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ReviewPage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("tripadvisor: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
