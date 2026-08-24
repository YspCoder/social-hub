package yelp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	MaximumPageSize           = 50
	MaximumOffset             = 1000
	MaximumSearchRadiusMeters = 40000
	maxProviderObjectBytes    = 8 << 20
)

// ResponseMeta preserves Yelp's plan-dependent daily quota headers as text.
// RateLimit-ResetTime is an ISO 8601 timestamp rather than a relative count.
type ResponseMeta struct {
	RequestID                   string
	RateLimitDailyLimit         string
	RateLimitRemaining          string
	RateLimitResourceDailyLimit string
	RateLimitResourceRemaining  string
	RateLimitResetTime          string
}

type BusinessSort string

const (
	SortBestMatch   BusinessSort = "best_match"
	SortRating      BusinessSort = "rating"
	SortReviewCount BusinessSort = "review_count"
	SortDistance    BusinessSort = "distance"
)

// BusinessAttribute is a non-Premium Search attribute documented by Yelp.
type BusinessAttribute string

const (
	AttributeHotAndNew             BusinessAttribute = "hot_and_new"
	AttributeRequestAQuote         BusinessAttribute = "request_a_quote"
	AttributeReservation           BusinessAttribute = "reservation"
	AttributeWaitlistReservation   BusinessAttribute = "waitlist_reservation"
	AttributeGenderNeutralRestroom BusinessAttribute = "gender_neutral_restrooms"
	AttributeOpenToAll             BusinessAttribute = "open_to_all"
	AttributeWheelchairAccessible  BusinessAttribute = "wheelchair_accessible"
)

// SearchBusinessesRequest accepts either Location or a complete coordinate
// pair. Radius and OpenNow are pointers because zero and false are documented
// query values distinct from omission.
type SearchBusinessesRequest struct {
	Location   string
	Latitude   *float64
	Longitude  *float64
	Term       string
	Radius     *int
	Categories []string
	Locale     string
	Price      []int
	OpenNow    *bool
	OpenAt     *int64
	Attributes []BusinessAttribute
	SortBy     BusinessSort
	Limit      int
	Offset     int
}

type GetBusinessRequest struct {
	BusinessIDOrAlias string
	Locale            string
}

type ListReviewsRequest struct {
	BusinessIDOrAlias string
	Locale            string
	Offset            int
	Limit             int
}

type ListCategoriesRequest struct {
	Locale string
}

// PlacesWorkflow exposes the bounded Yelp Places API v3 read surface.
type PlacesWorkflow interface {
	SearchBusinesses(context.Context, SearchBusinessesRequest, ...socialhub.CallOption) (SearchBusinessesResponse, error)
	GetBusiness(context.Context, GetBusinessRequest, ...socialhub.CallOption) (Business, error)
	ListReviews(context.Context, ListReviewsRequest, ...socialhub.CallOption) (ReviewsResponse, error)
	ListCategories(context.Context, ListCategoriesRequest, ...socialhub.CallOption) (CategoriesResponse, error)
}

type CategoryRef struct {
	Alias string `json:"alias"`
	Title string `json:"title"`
}

type Coordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type Location struct {
	Address1       string   `json:"address1"`
	Address2       string   `json:"address2"`
	Address3       string   `json:"address3"`
	City           string   `json:"city"`
	ZipCode        string   `json:"zip_code"`
	Country        string   `json:"country"`
	State          string   `json:"state"`
	DisplayAddress []string `json:"display_address"`
	CrossStreets   *string  `json:"cross_streets"`
}

type OpenPeriod struct {
	IsOvernight bool   `json:"is_overnight"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Day         int    `json:"day"`
}

// Hours normalizes the current hour_type field and the hours_type field still
// present in Yelp's official examples into Type.
type Hours struct {
	Type      string          `json:"hour_type"`
	Open      []OpenPeriod    `json:"open"`
	IsOpenNow bool            `json:"is_open_now"`
	Raw       json.RawMessage `json:"-"`
}

func (value *Hours) UnmarshalJSON(data []byte) error {
	var decoded struct {
		HourType  string       `json:"hour_type"`
		HoursType string       `json:"hours_type"`
		Open      []OpenPeriod `json:"open"`
		IsOpenNow bool         `json:"is_open_now"`
	}
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	value.Type = decoded.HourType
	if value.Type == "" {
		value.Type = decoded.HoursType
	}
	value.Open = decoded.Open
	value.IsOpenNow = decoded.IsOpenNow
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

// HoursList accepts the documented array and the singleton object used in
// Yelp's current Business Details example.
type HoursList []Hours

func (value *HoursList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		*value = nil
		return nil
	}
	switch {
	case len(trimmed) > 0 && trimmed[0] == '[':
		var decoded []Hours
		if err := json.Unmarshal(trimmed, &decoded); err != nil {
			return err
		}
		*value = decoded
		return nil
	case len(trimmed) > 0 && trimmed[0] == '{':
		var decoded Hours
		if err := json.Unmarshal(trimmed, &decoded); err != nil {
			return err
		}
		*value = []Hours{decoded}
		return nil
	default:
		return fmt.Errorf("yelp: hours must be an array, object, or null")
	}
}

type SpecialHours struct {
	Date        string `json:"date"`
	Start       string `json:"start"`
	End         string `json:"end"`
	IsOvernight bool   `json:"is_overnight"`
	IsClosed    *bool  `json:"is_closed"`
}

type OnlineReservation struct {
	URL string `json:"url"`
}

type Business struct {
	ID                string             `json:"id"`
	Alias             string             `json:"alias"`
	Name              string             `json:"name"`
	ImageURL          string             `json:"image_url"`
	IsClosed          bool               `json:"is_closed"`
	URL               string             `json:"url"`
	ReviewCount       int                `json:"review_count"`
	Rating            float64            `json:"rating"`
	Categories        []CategoryRef      `json:"categories"`
	Coordinates       Coordinates        `json:"coordinates"`
	Transactions      []string           `json:"transactions"`
	Price             string             `json:"price"`
	Location          Location           `json:"location"`
	Phone             string             `json:"phone"`
	DisplayPhone      string             `json:"display_phone"`
	Distance          *float64           `json:"distance"`
	IsClaimed         *bool              `json:"is_claimed"`
	DateOpened        string             `json:"date_opened"`
	DateClosed        string             `json:"date_closed"`
	Photos            []string           `json:"photos"`
	SpecialHours      []SpecialHours     `json:"special_hours"`
	Hours             HoursList          `json:"hours"`
	OnlineReservation *OnlineReservation `json:"online_reservation"`
	Meta              ResponseMeta       `json:"-"`
	Raw               json.RawMessage    `json:"-"`
}

func (value *Business) UnmarshalJSON(data []byte) error {
	type wire Business
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Business(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Region struct {
	Center Coordinates `json:"center"`
}

type SearchBusinessesResponse struct {
	Businesses []Business      `json:"businesses"`
	Total      int             `json:"total"`
	Region     Region          `json:"region"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *SearchBusinessesResponse) UnmarshalJSON(data []byte) error {
	type wire SearchBusinessesResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchBusinessesResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ReviewUser struct {
	ID         string  `json:"id"`
	ProfileURL string  `json:"profile_url"`
	ImageURL   *string `json:"image_url"`
	Name       string  `json:"name"`
}

type Review struct {
	ID          string          `json:"id"`
	URL         string          `json:"url"`
	Text        string          `json:"text"`
	Rating      int             `json:"rating"`
	TimeCreated string          `json:"time_created"`
	User        ReviewUser      `json:"user"`
	Raw         json.RawMessage `json:"-"`
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

type ReviewsResponse struct {
	Total             int             `json:"total"`
	Reviews           []Review        `json:"reviews"`
	PossibleLanguages []string        `json:"possible_languages"`
	Meta              ResponseMeta    `json:"-"`
	Raw               json.RawMessage `json:"-"`
}

func (value *ReviewsResponse) UnmarshalJSON(data []byte) error {
	type wire ReviewsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ReviewsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Category struct {
	Alias            string          `json:"alias"`
	Title            string          `json:"title"`
	ParentAliases    []string        `json:"parent_aliases"`
	CountryWhitelist []string        `json:"country_whitelist"`
	CountryBlacklist []string        `json:"country_blacklist"`
	Raw              json.RawMessage `json:"-"`
}

func (value *Category) UnmarshalJSON(data []byte) error {
	type wire Category
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Category(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CategoriesResponse struct {
	Categories []Category      `json:"categories"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *CategoriesResponse) UnmarshalJSON(data []byte) error {
	type wire CategoriesResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CategoriesResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("yelp: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
