package ebaybrowse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

type ResponseMeta struct {
	RequestID string
}

// RequestContext controls eBay marketplace localization, EPN attribution, and
// delivery estimates for one request.
type RequestContext struct {
	MarketplaceID        string
	AcceptLanguage       string
	AffiliateReferenceID string
	DeliveryCountry      string
	DeliveryPostalCode   string
}

type SearchSort string

const (
	SearchSortPrice           SearchSort = "price"
	SearchSortPriceDescending SearchSort = "-price"
	SearchSortDistance        SearchSort = "distance"
	SearchSortNewlyListed     SearchSort = "newlyListed"
	SearchSortEndingSoonest   SearchSort = "endingSoonest"
)

type SearchFieldGroup string

const (
	SearchFieldMatchingItems           SearchFieldGroup = "MATCHING_ITEMS"
	SearchFieldAspectRefinements       SearchFieldGroup = "ASPECT_REFINEMENTS"
	SearchFieldBuyingOptionRefinements SearchFieldGroup = "BUYING_OPTION_REFINEMENTS"
	SearchFieldCategoryRefinements     SearchFieldGroup = "CATEGORY_REFINEMENTS"
	SearchFieldConditionRefinements    SearchFieldGroup = "CONDITION_REFINEMENTS"
	SearchFieldExtended                SearchFieldGroup = "EXTENDED"
	SearchFieldFull                    SearchFieldGroup = "FULL"
)

type ItemFieldGroup string

const (
	ItemFieldProduct                 ItemFieldGroup = "PRODUCT"
	ItemFieldCompact                 ItemFieldGroup = "COMPACT"
	ItemFieldAdditionalSellerDetails ItemFieldGroup = "ADDITIONAL_SELLER_DETAILS"
	ItemFieldCharityDetails          ItemFieldGroup = "CHARITY_DETAILS"
)

type SearchItemsRequest struct {
	Query               string
	GTIN                string
	EPID                string
	CategoryID          string
	Filter              string
	AspectFilter        string
	CompatibilityFilter string
	FieldGroups         []SearchFieldGroup
	Sort                SearchSort
	Limit               int
	Offset              int
	Context             RequestContext
}

type GetItemRequest struct {
	ItemID                      string
	FieldGroups                 []ItemFieldGroup
	QuantityForShippingEstimate int
	Context                     RequestContext
}

type GetItemByLegacyIDRequest struct {
	LegacyItemID                string
	LegacyVariationID           string
	LegacyVariationSKU          string
	FieldGroups                 []ItemFieldGroup
	QuantityForShippingEstimate int
	Context                     RequestContext
}

type GetItemsByGroupRequest struct {
	ItemGroupID                 string
	FieldGroups                 []ItemFieldGroup
	QuantityForShippingEstimate int
	Context                     RequestContext
}

// BrowseWorkflow exposes the bounded eBay Buy Browse API v1 surface.
type BrowseWorkflow interface {
	SearchItems(context.Context, SearchItemsRequest, ...socialhub.CallOption) (SearchPage, error)
	GetItem(context.Context, GetItemRequest, ...socialhub.CallOption) (Item, error)
	GetItemByLegacyID(context.Context, GetItemByLegacyIDRequest, ...socialhub.CallOption) (Item, error)
	GetItemsByGroup(context.Context, GetItemsByGroupRequest, ...socialhub.CallOption) (ItemGroup, error)
}

// Amount preserves eBay's decimal and currency-conversion values as strings.
type Amount struct {
	Currency              string `json:"currency"`
	Value                 string `json:"value"`
	ConvertedFromCurrency string `json:"convertedFromCurrency"`
	ConvertedFromValue    string `json:"convertedFromValue"`
}

type Image struct {
	ImageURL string `json:"imageUrl"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type Category struct {
	CategoryID   string `json:"categoryId"`
	CategoryName string `json:"categoryName"`
}

type Seller struct {
	Username           string `json:"username"`
	UserID             string `json:"userId"`
	FeedbackPercentage string `json:"feedbackPercentage"`
	FeedbackScore      int    `json:"feedbackScore"`
	SellerAccountType  string `json:"sellerAccountType"`
}

type MarketingPrice struct {
	OriginalPrice      *Amount `json:"originalPrice"`
	DiscountAmount     *Amount `json:"discountAmount"`
	DiscountPercentage string  `json:"discountPercentage"`
	PriceTreatment     string  `json:"priceTreatment"`
}

type ShippingOption struct {
	Type                          string  `json:"type"`
	ShippingCostType              string  `json:"shippingCostType"`
	ShippingCost                  *Amount `json:"shippingCost"`
	AdditionalShippingCostPerUnit *Amount `json:"additionalShippingCostPerUnit"`
	ImportCharges                 *Amount `json:"importCharges"`
	ShippingCarrierCode           string  `json:"shippingCarrierCode"`
	ShippingServiceCode           string  `json:"shippingServiceCode"`
	MinEstimatedDeliveryDate      string  `json:"minEstimatedDeliveryDate"`
	MaxEstimatedDeliveryDate      string  `json:"maxEstimatedDeliveryDate"`
	GuaranteedDelivery            bool    `json:"guaranteedDelivery"`
	QuantityUsedForEstimate       int     `json:"quantityUsedForEstimate"`
}

type EstimatedAvailability struct {
	DeliveryOptions             []string `json:"deliveryOptions"`
	EstimatedAvailabilityStatus string   `json:"estimatedAvailabilityStatus"`
	EstimatedAvailableQuantity  int      `json:"estimatedAvailableQuantity"`
	EstimatedRemainingQuantity  int      `json:"estimatedRemainingQuantity"`
	EstimatedSoldQuantity       int      `json:"estimatedSoldQuantity"`
	AvailabilityThreshold       int      `json:"availabilityThreshold"`
	AvailabilityThresholdType   string   `json:"availabilityThresholdType"`
}

type ErrorParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ProviderError represents both eBay failure objects and successful-response warnings.
type ProviderError struct {
	ErrorID      int64            `json:"errorId"`
	Domain       string           `json:"domain"`
	Subdomain    string           `json:"subdomain"`
	Category     string           `json:"category"`
	Message      string           `json:"message"`
	LongMessage  string           `json:"longMessage"`
	Parameters   []ErrorParameter `json:"parameters"`
	InputRefIDs  []string         `json:"inputRefIds"`
	OutputRefIDs []string         `json:"outputRefIds"`
}

type ItemSummary struct {
	ItemID               string           `json:"itemId"`
	LegacyItemID         string           `json:"legacyItemId"`
	Title                string           `json:"title"`
	ShortDescription     string           `json:"shortDescription"`
	Condition            string           `json:"condition"`
	ConditionID          string           `json:"conditionId"`
	EPID                 string           `json:"epid"`
	Price                *Amount          `json:"price"`
	CurrentBidPrice      *Amount          `json:"currentBidPrice"`
	MarketingPrice       *MarketingPrice  `json:"marketingPrice"`
	Image                *Image           `json:"image"`
	AdditionalImages     []Image          `json:"additionalImages"`
	Categories           []Category       `json:"categories"`
	Seller               *Seller          `json:"seller"`
	ItemWebURL           string           `json:"itemWebUrl"`
	ItemAffiliateWebURL  string           `json:"itemAffiliateWebUrl"`
	ListingMarketplaceID string           `json:"listingMarketplaceId"`
	BuyingOptions        []string         `json:"buyingOptions"`
	ShippingOptions      []ShippingOption `json:"shippingOptions"`
	ItemCreationDate     string           `json:"itemCreationDate"`
	ItemEndDate          string           `json:"itemEndDate"`
	Raw                  json.RawMessage  `json:"-"`
}

func (value *ItemSummary) UnmarshalJSON(data []byte) error {
	type wire ItemSummary
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ItemSummary(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Item struct {
	ItemID                  string                  `json:"itemId"`
	LegacyItemID            string                  `json:"legacyItemId"`
	Title                   string                  `json:"title"`
	Subtitle                string                  `json:"subtitle"`
	ShortDescription        string                  `json:"shortDescription"`
	Description             string                  `json:"description"`
	Brand                   string                  `json:"brand"`
	GTIN                    string                  `json:"gtin"`
	EPID                    string                  `json:"epid"`
	MPN                     string                  `json:"mpn"`
	Condition               string                  `json:"condition"`
	ConditionID             string                  `json:"conditionId"`
	ConditionDescription    string                  `json:"conditionDescription"`
	CategoryID              string                  `json:"categoryId"`
	CategoryPath            string                  `json:"categoryPath"`
	Price                   *Amount                 `json:"price"`
	CurrentBidPrice         *Amount                 `json:"currentBidPrice"`
	MinimumPriceToBid       *Amount                 `json:"minimumPriceToBid"`
	MarketingPrice          *MarketingPrice         `json:"marketingPrice"`
	Image                   *Image                  `json:"image"`
	AdditionalImages        []Image                 `json:"additionalImages"`
	Seller                  *Seller                 `json:"seller"`
	ItemWebURL              string                  `json:"itemWebUrl"`
	ItemAffiliateWebURL     string                  `json:"itemAffiliateWebUrl"`
	ListingMarketplaceID    string                  `json:"listingMarketplaceId"`
	BuyingOptions           []string                `json:"buyingOptions"`
	ShippingOptions         []ShippingOption        `json:"shippingOptions"`
	EstimatedAvailabilities []EstimatedAvailability `json:"estimatedAvailabilities"`
	ItemCreationDate        string                  `json:"itemCreationDate"`
	ItemEndDate             string                  `json:"itemEndDate"`
	Warnings                []ProviderError         `json:"warnings"`
	Meta                    ResponseMeta            `json:"-"`
	Raw                     json.RawMessage         `json:"-"`
}

func (value *Item) UnmarshalJSON(data []byte) error {
	type wire Item
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Item(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type SearchPage struct {
	Items      []ItemSummary   `json:"itemSummaries"`
	Total      int             `json:"total"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
	Next       string          `json:"next"`
	Previous   string          `json:"prev"`
	Refinement json.RawMessage `json:"refinement"`
	Warnings   []ProviderError `json:"warnings"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *SearchPage) UnmarshalJSON(data []byte) error {
	type wire SearchPage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchPage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CommonDescription struct {
	Description string   `json:"description"`
	ItemIDs     []string `json:"itemIds"`
}

type ItemGroup struct {
	Items              []Item              `json:"items"`
	CommonDescriptions []CommonDescription `json:"commonDescriptions"`
	Warnings           []ProviderError     `json:"warnings"`
	Meta               ResponseMeta        `json:"-"`
	Raw                json.RawMessage     `json:"-"`
}

func (value *ItemGroup) UnmarshalJSON(data []byte) error {
	type wire ItemGroup
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ItemGroup(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("ebaybrowse: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
