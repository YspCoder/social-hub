package etsy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

type ListingState string

const (
	ListingActive   ListingState = "active"
	ListingInactive ListingState = "inactive"
	ListingSoldOut  ListingState = "sold_out"
	ListingDraft    ListingState = "draft"
	ListingExpired  ListingState = "expired"
)

type ListingType string

const (
	ListingPhysical ListingType = "physical"
	ListingDownload ListingType = "download"
	ListingBoth     ListingType = "both"
)

type WhoMade string

const (
	MadeByMe         WhoMade = "i_did"
	MadeByOther      WhoMade = "someone_else"
	MadeByCollective WhoMade = "collective"
)

type WhenMade string

const (
	MadeToOrder    WhenMade = "made_to_order"
	Made2020s      WhenMade = "2020_2026"
	Made2010s      WhenMade = "2010_2019"
	Made2007       WhenMade = "2007_2009"
	MadeBefore2007 WhenMade = "before_2007"
	Made2000s      WhenMade = "2000_2006"
	Made1990s      WhenMade = "1990s"
	Made1980s      WhenMade = "1980s"
	Made1970s      WhenMade = "1970s"
	Made1960s      WhenMade = "1960s"
	Made1950s      WhenMade = "1950s"
	Made1940s      WhenMade = "1940s"
	Made1930s      WhenMade = "1930s"
	Made1920s      WhenMade = "1920s"
	Made1910s      WhenMade = "1910s"
	Made1900s      WhenMade = "1900s"
	Made1800s      WhenMade = "1800s"
	Made1700s      WhenMade = "1700s"
	MadeBefore1700 WhenMade = "before_1700"
)

type WeightUnit string

const (
	WeightOunce    WeightUnit = "oz"
	WeightPound    WeightUnit = "lb"
	WeightGram     WeightUnit = "g"
	WeightKilogram WeightUnit = "kg"
)

type DimensionUnit string

const (
	DimensionInch       DimensionUnit = "in"
	DimensionFoot       DimensionUnit = "ft"
	DimensionMillimeter DimensionUnit = "mm"
	DimensionCentimeter DimensionUnit = "cm"
	DimensionMeter      DimensionUnit = "m"
	DimensionYard       DimensionUnit = "yd"
	DimensionInches     DimensionUnit = "inches"
)

type ListingSortField string

const (
	SortCreated ListingSortField = "created"
	SortPrice   ListingSortField = "price"
	SortUpdated ListingSortField = "updated"
	SortScore   ListingSortField = "score"
)

type SortOrder string

const (
	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

type ListingInclude string

const (
	IncludeShipping        ListingInclude = "Shipping"
	IncludeImages          ListingInclude = "Images"
	IncludeShop            ListingInclude = "Shop"
	IncludeUser            ListingInclude = "User"
	IncludeTranslations    ListingInclude = "Translations"
	IncludeInventory       ListingInclude = "Inventory"
	IncludeVideos          ListingInclude = "Videos"
	IncludePersonalization ListingInclude = "Personalization"
	IncludeBuyerPrice      ListingInclude = "BuyerPrice"
)

// ExactDecimal is an unsigned base-10 JSON number. It avoids float64 rounding
// in price-bearing write requests.
type ExactDecimal string

func (value ExactDecimal) MarshalJSON() ([]byte, error) {
	if !validExactDecimal(value, false) {
		return nil, fmt.Errorf("etsy: invalid exact decimal")
	}
	return []byte(value), nil
}

type ResponseMeta struct {
	RequestID           string
	LimitPerSecond      *int64
	RemainingThisSecond *int64
	LimitPerDay         *int64
	RemainingToday      *int64
}

type GetListingRequest struct {
	Includes            []ListingInclude
	Language            string
	AllowSuggestedTitle *bool
}

type ListShopListingsRequest struct {
	State     ListingState
	Limit     int
	Offset    int
	SortOn    ListingSortField
	SortOrder SortOrder
	Includes  []ListingInclude
}

// CreateDraftListingRequest cannot activate a listing. Publishing remains an
// explicit, out-of-scope operation.
type CreateDraftListingRequest struct {
	Quantity             int64
	Title                string
	Description          string
	Price                ExactDecimal
	WhoMade              WhoMade
	WhenMade             WhenMade
	TaxonomyID           int64
	ShippingProfileID    *int64
	ReturnPolicyID       *int64
	Materials            []string
	ShopSectionID        *int64
	ProcessingMin        *int64
	ProcessingMax        *int64
	ReadinessStateID     *int64
	Tags                 []string
	Styles               []string
	ItemWeight           *float64
	ItemLength           *float64
	ItemWidth            *float64
	ItemHeight           *float64
	ItemWeightUnit       WeightUnit
	ItemDimensionsUnit   DimensionUnit
	ProductionPartnerIDs []int64
	ImageIDs             []int64
	IsSupply             *bool
	IsCustomizable       *bool
	ShouldAutoRenew      *bool
	IsTaxable            *bool
	Type                 ListingType
}

type UploadListingImageRequest struct {
	Image          io.Reader
	FileName       string
	ListingImageID int64
	Rank           *int64
	Overwrite      *bool
	IsWatermarked  *bool
	AltText        *string
}

type GetListingInventoryRequest struct {
	ShowDeleted    *bool
	IncludeListing bool
}

type UpdateListingInventoryRequest struct {
	Products                 []InventoryProductInput `json:"products"`
	PriceOnProperty          []int64                 `json:"price_on_property"`
	QuantityOnProperty       []int64                 `json:"quantity_on_property"`
	SKUOnProperty            []int64                 `json:"sku_on_property"`
	ReadinessStateOnProperty []int64                 `json:"readiness_state_on_property"`
	MaxVariationsSupported   string                  `json:"-"`
}

type InventoryProductInput struct {
	SKU            string                   `json:"sku,omitempty"`
	PropertyValues []InventoryPropertyInput `json:"property_values"`
	Offerings      []InventoryOfferingInput `json:"offerings"`
}

type InventoryPropertyInput struct {
	PropertyID   int64    `json:"property_id"`
	ValueIDs     []int64  `json:"value_ids"`
	ScaleID      *int64   `json:"scale_id,omitempty"`
	PropertyName string   `json:"property_name,omitempty"`
	Values       []string `json:"values"`
}

type InventoryOfferingInput struct {
	Price            ExactDecimal `json:"price"`
	Quantity         int64        `json:"quantity"`
	IsEnabled        bool         `json:"is_enabled"`
	ReadinessStateID *int64       `json:"readiness_state_id"`
}

// ListingWorkflow is the bounded current Etsy seller surface.
type ListingWorkflow interface {
	GetShop(context.Context, ...socialhub.CallOption) (Shop, error)
	GetListing(context.Context, int64, GetListingRequest, ...socialhub.CallOption) (Listing, error)
	ListShopListings(context.Context, ListShopListingsRequest, ...socialhub.CallOption) (ListingsResponse, error)
	CreateDraftListing(context.Context, CreateDraftListingRequest, ...socialhub.CallOption) (Listing, error)
	ListListingImages(context.Context, int64, ...socialhub.CallOption) (ListingImagesResponse, error)
	UploadListingImage(context.Context, int64, UploadListingImageRequest, ...socialhub.CallOption) (ListingImage, error)
	GetListingInventory(context.Context, int64, GetListingInventoryRequest, ...socialhub.CallOption) (ListingInventory, error)
	UpdateListingInventory(context.Context, int64, UpdateListingInventoryRequest, ...socialhub.CallOption) (ListingInventory, error)
}

type Money struct {
	Amount       int64  `json:"amount"`
	Divisor      int64  `json:"divisor"`
	CurrencyCode string `json:"currency_code"`
}

type Shop struct {
	ShopID                    int64           `json:"shop_id"`
	UserID                    int64           `json:"user_id"`
	ShopName                  string          `json:"shop_name"`
	CreateDate                int64           `json:"create_date"`
	CreatedTimestamp          int64           `json:"created_timestamp"`
	Title                     *string         `json:"title"`
	Announcement              *string         `json:"announcement"`
	CurrencyCode              string          `json:"currency_code"`
	IsVacation                bool            `json:"is_vacation"`
	VacationMessage           *string         `json:"vacation_message"`
	SaleMessage               *string         `json:"sale_message"`
	DigitalSaleMessage        *string         `json:"digital_sale_message"`
	UpdateDate                int64           `json:"update_date"`
	UpdatedTimestamp          int64           `json:"updated_timestamp"`
	ListingActiveCount        int64           `json:"listing_active_count"`
	DigitalListingCount       int64           `json:"digital_listing_count"`
	LoginName                 string          `json:"login_name"`
	AcceptsCustomRequests     bool            `json:"accepts_custom_requests"`
	URL                       string          `json:"url"`
	ImageURL760x100           *string         `json:"image_url_760x100"`
	NumberOfFavorers          int64           `json:"num_favorers"`
	Languages                 []string        `json:"languages"`
	IconURLFullxFull          *string         `json:"icon_url_fullxfull"`
	IsDirectCheckoutOnboarded bool            `json:"is_direct_checkout_onboarded"`
	IsEtsyPaymentsOnboarded   bool            `json:"is_etsy_payments_onboarded"`
	TransactionSoldCount      int64           `json:"transaction_sold_count"`
	ShippingFromCountryISO    *string         `json:"shipping_from_country_iso"`
	ShopLocationCountryISO    *string         `json:"shop_location_country_iso"`
	ReviewCount               *int64          `json:"review_count"`
	ReviewAverage             *float64        `json:"review_average"`
	Meta                      ResponseMeta    `json:"-"`
	Raw                       json.RawMessage `json:"-"`
}

func (value *Shop) UnmarshalJSON(data []byte) error {
	type wire Shop
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Shop(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Listing struct {
	ListingID                 int64             `json:"listing_id"`
	UserID                    int64             `json:"user_id"`
	ShopID                    int64             `json:"shop_id"`
	Title                     string            `json:"title"`
	Description               string            `json:"description"`
	RichDescription           *string           `json:"rich_description"`
	State                     ListingState      `json:"state"`
	CreationTimestamp         int64             `json:"creation_timestamp"`
	CreatedTimestamp          int64             `json:"created_timestamp"`
	EndingTimestamp           int64             `json:"ending_timestamp"`
	OriginalCreationTimestamp int64             `json:"original_creation_timestamp"`
	LastModifiedTimestamp     int64             `json:"last_modified_timestamp"`
	UpdatedTimestamp          int64             `json:"updated_timestamp"`
	StateTimestamp            *int64            `json:"state_timestamp"`
	Quantity                  int64             `json:"quantity"`
	ShopSectionID             *int64            `json:"shop_section_id"`
	FeaturedRank              int64             `json:"featured_rank"`
	URL                       string            `json:"url"`
	NumberOfFavorers          int64             `json:"num_favorers"`
	NonTaxable                bool              `json:"non_taxable"`
	IsTaxable                 bool              `json:"is_taxable"`
	IsCustomizable            bool              `json:"is_customizable"`
	IsPersonalizable          bool              `json:"is_personalizable"`
	ListingType               ListingType       `json:"listing_type"`
	Tags                      []string          `json:"tags"`
	Materials                 []string          `json:"materials"`
	ShippingProfileID         *int64            `json:"shipping_profile_id"`
	ReturnPolicyID            *int64            `json:"return_policy_id"`
	ProcessingMin             *int64            `json:"processing_min"`
	ProcessingMax             *int64            `json:"processing_max"`
	WhoMade                   *WhoMade          `json:"who_made"`
	WhenMade                  *WhenMade         `json:"when_made"`
	IsSupply                  *bool             `json:"is_supply"`
	ItemWeight                *float64          `json:"item_weight"`
	ItemWeightUnit            *WeightUnit       `json:"item_weight_unit"`
	ItemLength                *float64          `json:"item_length"`
	ItemWidth                 *float64          `json:"item_width"`
	ItemHeight                *float64          `json:"item_height"`
	ItemDimensionsUnit        *DimensionUnit    `json:"item_dimensions_unit"`
	IsPrivate                 bool              `json:"is_private"`
	Style                     []string          `json:"style"`
	FileData                  *string           `json:"file_data"`
	HasVariations             bool              `json:"has_variations"`
	ShouldAutoRenew           bool              `json:"should_auto_renew"`
	Language                  *string           `json:"language"`
	Price                     Money             `json:"price"`
	ConvertedPrice            *Money            `json:"converted_price"`
	TaxonomyID                *int64            `json:"taxonomy_id"`
	ReadinessStateID          *int64            `json:"readiness_state_id"`
	SuggestedTitle            *string           `json:"suggested_title"`
	Shop                      *Shop             `json:"shop"`
	Images                    []ListingImage    `json:"images"`
	Inventory                 *ListingInventory `json:"inventory"`
	SKUs                      []string          `json:"skus"`
	Views                     int64             `json:"views"`
	Meta                      ResponseMeta      `json:"-"`
	Raw                       json.RawMessage   `json:"-"`
}

func (value *Listing) UnmarshalJSON(data []byte) error {
	type wire Listing
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Listing(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ListingsResponse struct {
	Count   int64           `json:"count"`
	Results []Listing       `json:"results"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

type ListingImage struct {
	ListingID        int64           `json:"listing_id"`
	ListingImageID   int64           `json:"listing_image_id"`
	HexCode          *string         `json:"hex_code"`
	Red              *int64          `json:"red"`
	Green            *int64          `json:"green"`
	Blue             *int64          `json:"blue"`
	Hue              *int64          `json:"hue"`
	Saturation       *int64          `json:"saturation"`
	Brightness       *int64          `json:"brightness"`
	IsBlackAndWhite  *bool           `json:"is_black_and_white"`
	CreationTSZ      int64           `json:"creation_tsz"`
	CreatedTimestamp int64           `json:"created_timestamp"`
	Rank             int64           `json:"rank"`
	URL75x75         string          `json:"url_75x75"`
	URL170x135       string          `json:"url_170x135"`
	URL570xN         string          `json:"url_570xN"`
	URLFullxFull     string          `json:"url_fullxfull"`
	FullHeight       *int64          `json:"full_height"`
	FullWidth        *int64          `json:"full_width"`
	AltText          *string         `json:"alt_text"`
	Meta             ResponseMeta    `json:"-"`
	Raw              json.RawMessage `json:"-"`
}

func (value *ListingImage) UnmarshalJSON(data []byte) error {
	type wire ListingImage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ListingImage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ListingImagesResponse struct {
	Count   int64           `json:"count"`
	Results []ListingImage  `json:"results"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

type ListingInventory struct {
	Products                 []InventoryProduct `json:"products"`
	PriceOnProperty          []int64            `json:"price_on_property"`
	QuantityOnProperty       []int64            `json:"quantity_on_property"`
	SKUOnProperty            []int64            `json:"sku_on_property"`
	ReadinessStateOnProperty []int64            `json:"readiness_state_on_property"`
	Listing                  *Listing           `json:"listing,omitempty"`
	Meta                     ResponseMeta       `json:"-"`
	Raw                      json.RawMessage    `json:"-"`
}

func (value *ListingInventory) UnmarshalJSON(data []byte) error {
	type wire ListingInventory
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ListingInventory(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type InventoryProduct struct {
	ProductID      int64                  `json:"product_id"`
	SKU            string                 `json:"sku"`
	IsDeleted      bool                   `json:"is_deleted"`
	Offerings      []InventoryOffering    `json:"offerings"`
	PropertyValues []ListingPropertyValue `json:"property_values"`
}

type InventoryOffering struct {
	OfferingID       int64  `json:"offering_id"`
	Quantity         int64  `json:"quantity"`
	IsEnabled        bool   `json:"is_enabled"`
	IsDeleted        bool   `json:"is_deleted"`
	Price            Money  `json:"price"`
	ReadinessStateID *int64 `json:"readiness_state_id"`
}

type ListingPropertyValue struct {
	PropertyID   int64    `json:"property_id"`
	PropertyName *string  `json:"property_name"`
	ScaleID      *int64   `json:"scale_id"`
	ScaleName    *string  `json:"scale_name"`
	ValueIDs     []int64  `json:"value_ids"`
	Values       []string `json:"values"`
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("etsy: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
