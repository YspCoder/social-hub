package googleplaces

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumSearchPageSize = 20
	MaximumNearbyTypes    = 50
	MaximumPhotoDimension = 4800
	maxProviderObjectSize = 8 << 20
)

// PlaceField is an implemented Place response field. Wildcard masks and raw
// field strings are deliberately unsupported because fields select billing
// SKUs in Places API (New).
type PlaceField string

const (
	FieldID                       PlaceField = "id"
	FieldResourceName             PlaceField = "name"
	FieldDisplayName              PlaceField = "displayName"
	FieldFormattedAddress         PlaceField = "formattedAddress"
	FieldShortFormattedAddress    PlaceField = "shortFormattedAddress"
	FieldLocation                 PlaceField = "location"
	FieldViewport                 PlaceField = "viewport"
	FieldTypes                    PlaceField = "types"
	FieldPrimaryType              PlaceField = "primaryType"
	FieldPrimaryTypeDisplayName   PlaceField = "primaryTypeDisplayName"
	FieldBusinessStatus           PlaceField = "businessStatus"
	FieldGoogleMapsURI            PlaceField = "googleMapsUri"
	FieldWebsiteURI               PlaceField = "websiteUri"
	FieldInternationalPhoneNumber PlaceField = "internationalPhoneNumber"
	FieldNationalPhoneNumber      PlaceField = "nationalPhoneNumber"
	FieldRating                   PlaceField = "rating"
	FieldUserRatingCount          PlaceField = "userRatingCount"
	FieldPriceLevel               PlaceField = "priceLevel"
	FieldUTCOffsetMinutes         PlaceField = "utcOffsetMinutes"
	FieldPhotos                   PlaceField = "photos"
	FieldAttributions             PlaceField = "attributions"
)

type PriceLevel string

const (
	PriceLevelUnspecified   PriceLevel = "PRICE_LEVEL_UNSPECIFIED"
	PriceLevelFree          PriceLevel = "PRICE_LEVEL_FREE"
	PriceLevelInexpensive   PriceLevel = "PRICE_LEVEL_INEXPENSIVE"
	PriceLevelModerate      PriceLevel = "PRICE_LEVEL_MODERATE"
	PriceLevelExpensive     PriceLevel = "PRICE_LEVEL_EXPENSIVE"
	PriceLevelVeryExpensive PriceLevel = "PRICE_LEVEL_VERY_EXPENSIVE"
)

type BusinessStatus string

const (
	BusinessStatusUnspecified       BusinessStatus = "BUSINESS_STATUS_UNSPECIFIED"
	BusinessStatusOperational       BusinessStatus = "OPERATIONAL"
	BusinessStatusClosedTemporarily BusinessStatus = "CLOSED_TEMPORARILY"
	BusinessStatusClosedPermanently BusinessStatus = "CLOSED_PERMANENTLY"
	BusinessStatusFutureOpening     BusinessStatus = "FUTURE_OPENING"
)

type TextRankPreference string

const (
	TextRankDistance  TextRankPreference = "DISTANCE"
	TextRankRelevance TextRankPreference = "RELEVANCE"
)

type NearbyRankPreference string

const (
	NearbyRankDistance   NearbyRankPreference = "DISTANCE"
	NearbyRankPopularity NearbyRankPreference = "POPULARITY"
)

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Circle struct {
	Center LatLng  `json:"center"`
	Radius float64 `json:"radius"`
}

type Viewport struct {
	Low  LatLng `json:"low"`
	High LatLng `json:"high"`
}

type LocationBias struct {
	Circle    *Circle   `json:"circle,omitempty"`
	Rectangle *Viewport `json:"rectangle,omitempty"`
}

type TextLocationRestriction struct {
	Rectangle Viewport `json:"rectangle"`
}

type NearbyLocationRestriction struct {
	Circle Circle `json:"circle"`
}

type TextSearchRequest struct {
	TextQuery                        string                   `json:"textQuery"`
	LanguageCode                     string                   `json:"languageCode,omitempty"`
	RegionCode                       string                   `json:"regionCode,omitempty"`
	RankPreference                   TextRankPreference       `json:"rankPreference,omitempty"`
	IncludedType                     string                   `json:"includedType,omitempty"`
	OpenNow                          *bool                    `json:"openNow,omitempty"`
	MinRating                        *float64                 `json:"minRating,omitempty"`
	PriceLevels                      []PriceLevel             `json:"priceLevels,omitempty"`
	StrictTypeFiltering              *bool                    `json:"strictTypeFiltering,omitempty"`
	LocationBias                     *LocationBias            `json:"locationBias,omitempty"`
	LocationRestriction              *TextLocationRestriction `json:"locationRestriction,omitempty"`
	IncludePureServiceAreaBusinesses *bool                    `json:"includePureServiceAreaBusinesses,omitempty"`
	IncludeFutureOpeningBusinesses   *bool                    `json:"includeFutureOpeningBusinesses,omitempty"`
	PageSize                         int                      `json:"pageSize,omitempty"`
	PageToken                        string                   `json:"pageToken,omitempty"`
	Fields                           []PlaceField             `json:"-"`
	IncludeNextPageToken             bool                     `json:"-"`
}

type NearbySearchRequest struct {
	LanguageCode                   string                    `json:"languageCode,omitempty"`
	RegionCode                     string                    `json:"regionCode,omitempty"`
	IncludedTypes                  []string                  `json:"includedTypes,omitempty"`
	ExcludedTypes                  []string                  `json:"excludedTypes,omitempty"`
	IncludedPrimaryTypes           []string                  `json:"includedPrimaryTypes,omitempty"`
	ExcludedPrimaryTypes           []string                  `json:"excludedPrimaryTypes,omitempty"`
	MaxResultCount                 int                       `json:"maxResultCount,omitempty"`
	LocationRestriction            NearbyLocationRestriction `json:"locationRestriction"`
	RankPreference                 NearbyRankPreference      `json:"rankPreference,omitempty"`
	IncludeFutureOpeningBusinesses *bool                     `json:"includeFutureOpeningBusinesses,omitempty"`
	Fields                         []PlaceField              `json:"-"`
}

type GetPlaceRequest struct {
	PlaceID      string
	LanguageCode string
	RegionCode   string
	SessionToken string
	Fields       []PlaceField
}

type GetPhotoMediaRequest struct {
	PhotoName   string
	MaxWidthPx  int
	MaxHeightPx int
}

// PlacesWorkflow exposes the current minimal Places API (New) read surface.
type PlacesWorkflow interface {
	TextSearch(context.Context, TextSearchRequest, ...socialhub.CallOption) (PlacePage, error)
	NearbySearch(context.Context, NearbySearchRequest, ...socialhub.CallOption) (PlacePage, error)
	GetPlace(context.Context, GetPlaceRequest, ...socialhub.CallOption) (Place, error)
	GetPhotoMedia(context.Context, GetPhotoMediaRequest, ...socialhub.CallOption) (PhotoMedia, error)
}

// ResponseMeta preserves request correlation, retry, and dynamic quota
// headers without assigning them fixed semantics.
type ResponseMeta struct {
	RequestID        string
	TraceContext     string
	RetryAfterHeader string
	RetryAfter       time.Duration
	QuotaHeaders     map[string]string
}

type LocalizedText struct {
	Text         string `json:"text"`
	LanguageCode string `json:"languageCode"`
}

type AuthorAttribution struct {
	DisplayName string `json:"displayName"`
	URI         string `json:"uri"`
	PhotoURI    string `json:"photoUri"`
}

type Photo struct {
	Name               string              `json:"name"`
	WidthPx            int                 `json:"widthPx"`
	HeightPx           int                 `json:"heightPx"`
	AuthorAttributions []AuthorAttribution `json:"authorAttributions"`
	FlagContentURI     string              `json:"flagContentUri"`
	GoogleMapsURI      string              `json:"googleMapsUri"`
	Raw                json.RawMessage     `json:"-"`
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

type Attribution struct {
	Provider    string `json:"provider"`
	ProviderURI string `json:"providerUri"`
}

type Place struct {
	ID                       string          `json:"id"`
	Name                     string          `json:"name"`
	DisplayName              *LocalizedText  `json:"displayName"`
	FormattedAddress         string          `json:"formattedAddress"`
	ShortFormattedAddress    string          `json:"shortFormattedAddress"`
	Location                 *LatLng         `json:"location"`
	Viewport                 *Viewport       `json:"viewport"`
	Types                    []string        `json:"types"`
	PrimaryType              string          `json:"primaryType"`
	PrimaryTypeDisplayName   *LocalizedText  `json:"primaryTypeDisplayName"`
	BusinessStatus           BusinessStatus  `json:"businessStatus"`
	GoogleMapsURI            string          `json:"googleMapsUri"`
	WebsiteURI               string          `json:"websiteUri"`
	InternationalPhoneNumber string          `json:"internationalPhoneNumber"`
	NationalPhoneNumber      string          `json:"nationalPhoneNumber"`
	Rating                   *float64        `json:"rating"`
	UserRatingCount          *int            `json:"userRatingCount"`
	PriceLevel               PriceLevel      `json:"priceLevel"`
	UTCOffsetMinutes         *int            `json:"utcOffsetMinutes"`
	Photos                   []Photo         `json:"photos"`
	Attributions             []Attribution   `json:"attributions"`
	Meta                     ResponseMeta    `json:"-"`
	Raw                      json.RawMessage `json:"-"`
}

func (value *Place) UnmarshalJSON(data []byte) error {
	type wire Place
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Place(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PlacePage struct {
	Places        []Place         `json:"places"`
	NextPageToken string          `json:"nextPageToken"`
	Meta          ResponseMeta    `json:"-"`
	Raw           json.RawMessage `json:"-"`
}

func (value *PlacePage) UnmarshalJSON(data []byte) error {
	type wire PlacePage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PlacePage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PhotoMedia struct {
	Name     string          `json:"name"`
	PhotoURI string          `json:"photoUri"`
	Meta     ResponseMeta    `json:"-"`
	Raw      json.RawMessage `json:"-"`
}

func (value *PhotoMedia) UnmarshalJSON(data []byte) error {
	type wire PhotoMedia
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PhotoMedia(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectSize || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("googleplaces: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
