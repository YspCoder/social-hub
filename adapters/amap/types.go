package amap

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	MaximumPageSize           = 25
	MaximumSearchWindow       = 200
	MaximumAroundRadiusMeters = 50_000
	MaximumDetailIDs          = 10
	maxProviderObjectBytes    = 8 << 20
)

// QuotaDocumentationURL points to Amap's dynamic quota guidance. Exact QPS
// and call allowances must be read from the account's console.
const QuotaDocumentationURL = quotaURL

// TermsDocumentationURL points to the current Amap Open Platform Service
// Agreement governing certification, licensing, and place-data use.
const TermsDocumentationURL = termsURL

type Language string

const (
	LanguageChinese Language = "zh"
	LanguageEnglish Language = "en"
)

type ShowField string

const (
	ShowChildren ShowField = "children"
	ShowBusiness ShowField = "business"
	ShowIndoor   ShowField = "indoor"
	ShowNavi     ShowField = "navi"
	ShowPhotos   ShowField = "photos"
)

type AroundSort string

const (
	AroundSortDistance AroundSort = "distance"
	AroundSortWeight   AroundSort = "weight"
)

// Coordinate is an Amap longitude/latitude pair. Place Search v5 inputs and
// outputs use Amap coordinates, which are GCJ-02 in mainland China.
type Coordinate struct {
	Longitude float64
	Latitude  float64
}

func (value Coordinate) String() string {
	return formatCoordinateNumber(value.Longitude) + "," + formatCoordinateNumber(value.Latitude)
}

func (value *Coordinate) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("amap: coordinate must be a longitude,latitude string: %w", err)
	}
	longitudeText, latitudeText, found := strings.Cut(text, ",")
	if !found || strings.Contains(latitudeText, ",") {
		return fmt.Errorf("amap: coordinate must contain one comma")
	}
	longitude, err := strconv.ParseFloat(longitudeText, 64)
	if err != nil {
		return fmt.Errorf("amap: invalid longitude")
	}
	latitude, err := strconv.ParseFloat(latitudeText, 64)
	if err != nil {
		return fmt.Errorf("amap: invalid latitude")
	}
	decoded := Coordinate{Longitude: longitude, Latitude: latitude}
	if !validCoordinate(decoded) {
		return fmt.Errorf("amap: coordinate is outside longitude/latitude bounds")
	}
	*value = decoded
	return nil
}

func formatCoordinateNumber(value float64) string {
	text := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
	if text == "-0" {
		return "0"
	}
	return text
}

func validCoordinate(value Coordinate) bool {
	return !math.IsNaN(value.Longitude) && !math.IsInf(value.Longitude, 0) &&
		!math.IsNaN(value.Latitude) && !math.IsInf(value.Latitude, 0) &&
		value.Longitude >= -180 && value.Longitude <= 180 && value.Latitude >= -90 && value.Latitude <= 90
}

type TextSearchRequest struct {
	Keywords   string
	TypeCodes  []string
	Region     string
	Language   Language
	CityLimit  *bool
	ShowFields []ShowField
	PageSize   int
	PageNumber int
}

type AroundSearchRequest struct {
	Keywords  string
	TypeCodes []string
	Location  Coordinate
	// Radius 0 keeps Amap's provider default; explicit values are in meters.
	Radius     int
	Sort       AroundSort
	Region     string
	Language   Language
	CityLimit  *bool
	ShowFields []ShowField
	PageSize   int
	PageNumber int
}

type DetailRequest struct {
	IDs        []string
	Language   Language
	ShowFields []ShowField
}

type ChildPlace struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Location *Coordinate `json:"location"`
	Address  string      `json:"address"`
	Subtype  string      `json:"subtype"`
	TypeCode string      `json:"typecode"`
	TypeName string      `json:"sname"`
}

type Business struct {
	BusinessArea      string `json:"business_area"`
	OpenToday         string `json:"opentime_today"`
	OpenWeek          string `json:"opentime_week"`
	Telephone         string `json:"tel"`
	Tag               string `json:"tag"`
	Rating            string `json:"rating"`
	Cost              string `json:"cost"`
	ParkingType       string `json:"parking_type"`
	Alias             string `json:"alias"`
	KeyTag            string `json:"keytag"`
	RecommendationTag string `json:"rectag"`
}

type Indoor struct {
	HasIndoorMap string `json:"indoor_map"`
	ParentID     string `json:"cpid"`
	Floor        string `json:"floor"`
	TrueFloor    string `json:"truefloor"`
}

type Navigation struct {
	NavigationPOIID string      `json:"navi_poiid"`
	Entrance        *Coordinate `json:"entr_location"`
	Exit            *Coordinate `json:"exit_location"`
	GridCode        string      `json:"gridcode"`
}

type Photo struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Place struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	ParentID       string          `json:"parent"`
	DistanceMeters string          `json:"distance"`
	Location       *Coordinate     `json:"location"`
	Type           string          `json:"type"`
	TypeCode       string          `json:"typecode"`
	ProvinceName   string          `json:"pname"`
	CityName       string          `json:"cityname"`
	DistrictName   string          `json:"adname"`
	Address        string          `json:"address"`
	ProvinceCode   string          `json:"pcode"`
	CityCode       string          `json:"citycode"`
	DistrictCode   string          `json:"adcode"`
	CategoryTag    string          `json:"atag"`
	Children       []ChildPlace    `json:"children"`
	Business       *Business       `json:"business"`
	Indoor         *Indoor         `json:"indoor"`
	Navigation     *Navigation     `json:"navi"`
	Photos         []Photo         `json:"photos"`
	Raw            json.RawMessage `json:"-"`
}

func (value *Place) UnmarshalJSON(data []byte) error {
	type wire Place
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Place(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type ResponseMeta struct {
	HTTPStatus int
	Status     string
	Info       string
	InfoCode   string
	Count      int
	PageSize   int
	PageNumber int
}

type PlacePage struct {
	Places []Place
	Meta   ResponseMeta
	Raw    json.RawMessage
}

type PlaceDetails struct {
	Places []Place
	Meta   ResponseMeta
	Raw    json.RawMessage
}

type PlacesWorkflow interface {
	SearchText(context.Context, TextSearchRequest, ...socialhub.CallOption) (PlacePage, error)
	SearchAround(context.Context, AroundSearchRequest, ...socialhub.CallOption) (PlacePage, error)
	GetDetails(context.Context, DetailRequest, ...socialhub.CallOption) (PlaceDetails, error)
}

func decodeProviderObject(data []byte, target any) error {
	if len(data) == 0 || len(data) > maxProviderObjectBytes || !json.Valid(data) {
		return fmt.Errorf("amap: invalid provider object")
	}
	return json.Unmarshal(data, target)
}

var _ PlacesWorkflow = (*Client)(nil)
