package foursquare

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

// Sort selects the order of place search results.
type Sort string

const (
	SortRelevance  Sort = "RELEVANCE"
	SortRating     Sort = "RATING"
	SortDistance   Sort = "DISTANCE"
	SortPopularity Sort = "POPULARITY"
)

// PlaceField is a Pro response field accepted by this adapter. Premium fields
// are deliberately excluded because requesting one changes request pricing.
type PlaceField string

const (
	FieldFSQPlaceID       PlaceField = "fsq_place_id"
	FieldName             PlaceField = "name"
	FieldCategories       PlaceField = "categories"
	FieldLocation         PlaceField = "location"
	FieldLatitude         PlaceField = "latitude"
	FieldLongitude        PlaceField = "longitude"
	FieldDistance         PlaceField = "distance"
	FieldTelephone        PlaceField = "tel"
	FieldEmail            PlaceField = "email"
	FieldWebsite          PlaceField = "website"
	FieldSocialMedia      PlaceField = "social_media"
	FieldLink             PlaceField = "link"
	FieldDateClosed       PlaceField = "date_closed"
	FieldPlacemakerURL    PlaceField = "placemaker_url"
	FieldChains           PlaceField = "chains"
	FieldStoreID          PlaceField = "store_id"
	FieldRelatedPlaces    PlaceField = "related_places"
	FieldExtendedLocation PlaceField = "extended_location"
	FieldUnresolvedFlags  PlaceField = "unresolved_flags"
)

// Coordinate is a latitude/longitude pair.
type Coordinate struct {
	Latitude  float64
	Longitude float64
}

// SearchRequest describes a Places API search. At most one location mode may
// be set: LL, Near, or a NorthEast/SouthWest rectangle. With no mode, the API
// may apply its documented IP bias.
type SearchRequest struct {
	Query       string
	LL          *Coordinate
	Radius      int
	Near        string
	NorthEast   *Coordinate
	SouthWest   *Coordinate
	CategoryIDs []string
	Sort        Sort
	Limit       int
	Cursor      string
	Fields      []PlaceField
}

// Image describes an icon or chain logo returned by Foursquare.
type Image struct {
	ID              string   `json:"id"`
	CreatedAt       string   `json:"created_at"`
	Prefix          string   `json:"prefix"`
	Suffix          string   `json:"suffix"`
	Width           int      `json:"width"`
	Height          int      `json:"height"`
	Classifications []string `json:"classifications"`
}

// Category is a Foursquare place category.
type Category struct {
	FSQCategoryID string `json:"fsq_category_id"`
	Name          string `json:"name"`
	ShortName     string `json:"short_name"`
	PluralName    string `json:"plural_name"`
	Icon          *Image `json:"icon"`
}

// Location is the structured postal location of a place.
type Location struct {
	Address          string `json:"address"`
	Locality         string `json:"locality"`
	Region           string `json:"region"`
	Postcode         string `json:"postcode"`
	AdminRegion      string `json:"admin_region"`
	PostTown         string `json:"post_town"`
	POBox            string `json:"po_box"`
	Country          string `json:"country"`
	FormattedAddress string `json:"formatted_address"`
}

// Chain identifies a brand chain associated with a place.
type Chain struct {
	FSQChainID string `json:"fsq_chain_id"`
	Name       string `json:"name"`
	Logo       *Image `json:"logo"`
	ParentID   string `json:"parent_id"`
}

// SocialMedia contains provider handles embedded in a place.
type SocialMedia struct {
	FacebookID string `json:"facebook_id"`
	Instagram  string `json:"instagram"`
	Twitter    string `json:"twitter"`
}

// RelatedPlace is the stable subset of a parent or child place reference.
type RelatedPlace struct {
	FSQPlaceID string     `json:"fsq_place_id"`
	Name       string     `json:"name"`
	Categories []Category `json:"categories"`
}

// RelatedPlaces groups a place's parent and child relationships.
type RelatedPlaces struct {
	Parent   *RelatedPlace  `json:"parent"`
	Children []RelatedPlace `json:"children"`
}

// ExtendedLocation contains administrative identifiers outside Location.
type ExtendedLocation struct {
	DMA           string `json:"dma"`
	CensusBlockID string `json:"census_block_id"`
}

// Place is the stable Pro-field subset of a Foursquare place. Pointer scalar
// fields distinguish an omitted field from a legitimate zero value.
type Place struct {
	FSQPlaceID       string            `json:"fsq_place_id"`
	Name             string            `json:"name"`
	Latitude         *float64          `json:"latitude"`
	Longitude        *float64          `json:"longitude"`
	Distance         *int              `json:"distance"`
	Categories       []Category        `json:"categories"`
	Location         *Location         `json:"location"`
	Chains           []Chain           `json:"chains"`
	Telephone        string            `json:"tel"`
	Email            string            `json:"email"`
	Website          string            `json:"website"`
	SocialMedia      *SocialMedia      `json:"social_media"`
	Link             string            `json:"link"`
	DateClosed       string            `json:"date_closed"`
	PlacemakerURL    string            `json:"placemaker_url"`
	StoreID          string            `json:"store_id"`
	RelatedPlaces    *RelatedPlaces    `json:"related_places"`
	ExtendedLocation *ExtendedLocation `json:"extended_location"`
	UnresolvedFlags  []string          `json:"unresolved_flags"`
	RequestID        string            `json:"-"`
	Raw              json.RawMessage   `json:"-"`
}

// UnmarshalJSON decodes stable fields while retaining the complete entity.
func (place *Place) UnmarshalJSON(data []byte) error {
	type wirePlace Place
	var decoded wirePlace
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*place = Place(decoded)
	place.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// PlacePage is one cursor-paginated search response.
type PlacePage struct {
	Places     []Place
	NextCursor *string
	Link       string
	RequestID  string
	Raw        json.RawMessage
}

// PlacesWorkflow exposes the current minimal Places API read surface.
type PlacesWorkflow interface {
	SearchPlaces(context.Context, SearchRequest, ...socialhub.CallOption) (*PlacePage, error)
	GetPlace(context.Context, string, ...socialhub.CallOption) (*Place, error)
}
