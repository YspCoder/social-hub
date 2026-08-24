package amazoncreators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	maxExactValueBytes     = 256
	maxProviderObjectBytes = 8 << 20
)

type ResponseMeta struct {
	RequestID string
}

type Availability string

const (
	AvailabilityAvailable         Availability = "Available"
	AvailabilityIncludeOutOfStock Availability = "IncludeOutOfStock"
)

type Condition string

const (
	ConditionAny Condition = "Any"
	ConditionNew Condition = "New"
)

type DeliveryFlag string

const (
	DeliveryAmazonGlobal      DeliveryFlag = "AmazonGlobal"
	DeliveryFreeShipping      DeliveryFlag = "FreeShipping"
	DeliveryFulfilledByAmazon DeliveryFlag = "FulfilledByAmazon"
	DeliveryPrime             DeliveryFlag = "Prime"
)

type SortBy string

const (
	SortAverageCustomerReviews SortBy = "AvgCustomerReviews"
	SortFeatured               SortBy = "Featured"
	SortNewestArrivals         SortBy = "NewestArrivals"
	SortPriceHighToLow         SortBy = "Price:HighToLow"
	SortPriceLowToHigh         SortBy = "Price:LowToHigh"
	SortRelevance              SortBy = "Relevance"
)

// Resource is a current Creators API Catalog resource selector. Availability
// is operation-specific and is checked before a request is sent.
type Resource string

const (
	ResourceBrowseNodeInfo               Resource = "browseNodeInfo.browseNodes"
	ResourceBrowseNodeInfoAncestor       Resource = "browseNodeInfo.browseNodes.ancestor"
	ResourceBrowseNodeInfoSalesRank      Resource = "browseNodeInfo.browseNodes.salesRank"
	ResourceWebsiteSalesRank             Resource = "browseNodeInfo.websiteSalesRank"
	ResourceCustomerReviewsCount         Resource = "customerReviews.count"
	ResourceCustomerReviewsStarRating    Resource = "customerReviews.starRating"
	ResourceImagesPrimarySmall           Resource = "images.primary.small"
	ResourceImagesPrimaryMedium          Resource = "images.primary.medium"
	ResourceImagesPrimaryLarge           Resource = "images.primary.large"
	ResourceImagesPrimaryHighResolution  Resource = "images.primary.highRes"
	ResourceImagesVariantsSmall          Resource = "images.variants.small"
	ResourceImagesVariantsMedium         Resource = "images.variants.medium"
	ResourceImagesVariantsLarge          Resource = "images.variants.large"
	ResourceImagesVariantsHighResolution Resource = "images.variants.highRes"
	ResourceItemInfoByLine               Resource = "itemInfo.byLineInfo"
	ResourceItemInfoContent              Resource = "itemInfo.contentInfo"
	ResourceItemInfoContentRating        Resource = "itemInfo.contentRating"
	ResourceItemInfoClassifications      Resource = "itemInfo.classifications"
	ResourceItemInfoExternalIDs          Resource = "itemInfo.externalIds"
	ResourceItemInfoFeatures             Resource = "itemInfo.features"
	ResourceItemInfoManufacture          Resource = "itemInfo.manufactureInfo"
	ResourceItemInfoProduct              Resource = "itemInfo.productInfo"
	ResourceItemInfoTechnical            Resource = "itemInfo.technicalInfo"
	ResourceItemInfoTitle                Resource = "itemInfo.title"
	ResourceItemInfoTradeIn              Resource = "itemInfo.tradeInInfo"
	ResourceOffersAvailability           Resource = "offersV2.listings.availability"
	ResourceOffersCondition              Resource = "offersV2.listings.condition"
	ResourceOffersDealDetails            Resource = "offersV2.listings.dealDetails"
	ResourceOffersBuyBoxWinner           Resource = "offersV2.listings.isBuyBoxWinner"
	ResourceOffersLoyaltyPoints          Resource = "offersV2.listings.loyaltyPoints"
	ResourceOffersMerchantInfo           Resource = "offersV2.listings.merchantInfo"
	ResourceOffersPrice                  Resource = "offersV2.listings.price"
	ResourceOffersType                   Resource = "offersV2.listings.type"
	ResourceParentASIN                   Resource = "parentASIN"
	ResourceSearchRefinements            Resource = "searchRefinements"
	ResourceVariationSummaryHighestPrice Resource = "variationSummary.price.highestPrice"
	ResourceVariationSummaryLowestPrice  Resource = "variationSummary.price.lowestPrice"
	ResourceVariationSummaryDimension    Resource = "variationSummary.variationDimension"
	ResourceBrowseNodesAncestor          Resource = "browseNodes.ancestor"
	ResourceBrowseNodesChildren          Resource = "browseNodes.children"
)

var sharedItemResources = []Resource{
	ResourceBrowseNodeInfo, ResourceBrowseNodeInfoAncestor, ResourceBrowseNodeInfoSalesRank,
	ResourceWebsiteSalesRank, ResourceCustomerReviewsCount, ResourceCustomerReviewsStarRating,
	ResourceImagesPrimarySmall, ResourceImagesPrimaryMedium, ResourceImagesPrimaryLarge,
	ResourceImagesPrimaryHighResolution, ResourceImagesVariantsSmall, ResourceImagesVariantsMedium,
	ResourceImagesVariantsLarge, ResourceImagesVariantsHighResolution, ResourceItemInfoByLine,
	ResourceItemInfoContent, ResourceItemInfoContentRating, ResourceItemInfoClassifications,
	ResourceItemInfoExternalIDs, ResourceItemInfoFeatures, ResourceItemInfoManufacture,
	ResourceItemInfoProduct, ResourceItemInfoTechnical, ResourceItemInfoTitle, ResourceItemInfoTradeIn,
	ResourceOffersAvailability, ResourceOffersCondition, ResourceOffersDealDetails,
	ResourceOffersBuyBoxWinner, ResourceOffersLoyaltyPoints, ResourceOffersMerchantInfo,
	ResourceOffersPrice, ResourceOffersType, ResourceParentASIN,
}

type SearchItemsRequest struct {
	Actor                 string            `json:"actor,omitempty"`
	Artist                string            `json:"artist,omitempty"`
	Author                string            `json:"author,omitempty"`
	Availability          Availability      `json:"availability,omitempty"`
	Brand                 string            `json:"brand,omitempty"`
	BrowseNodeID          string            `json:"browseNodeId,omitempty"`
	Condition             Condition         `json:"condition,omitempty"`
	CurrencyOfPreference  string            `json:"currencyOfPreference,omitempty"`
	DeliveryFlags         []DeliveryFlag    `json:"deliveryFlags,omitempty"`
	ItemCount             int               `json:"itemCount,omitempty"`
	ItemPage              int               `json:"itemPage,omitempty"`
	Keywords              string            `json:"keywords,omitempty"`
	LanguagesOfPreference []string          `json:"languagesOfPreference,omitempty"`
	MaxPrice              int64             `json:"maxPrice,omitempty"`
	MinPrice              int64             `json:"minPrice,omitempty"`
	MinReviewsRating      float64           `json:"minReviewsRating,omitempty"`
	MinSavingPercent      int               `json:"minSavingPercent,omitempty"`
	Properties            map[string]string `json:"properties,omitempty"`
	Resources             []Resource        `json:"resources,omitempty"`
	SearchIndex           string            `json:"searchIndex,omitempty"`
	SortBy                SortBy            `json:"sortBy,omitempty"`
	Title                 string            `json:"title,omitempty"`
}

type GetItemsRequest struct {
	ItemIDs               []string          `json:"itemIds"`
	Condition             Condition         `json:"condition,omitempty"`
	CurrencyOfPreference  string            `json:"currencyOfPreference,omitempty"`
	LanguagesOfPreference []string          `json:"languagesOfPreference,omitempty"`
	Properties            map[string]string `json:"properties,omitempty"`
	Resources             []Resource        `json:"resources,omitempty"`
}

type GetVariationsRequest struct {
	ASIN                  string            `json:"asin"`
	Condition             Condition         `json:"condition,omitempty"`
	CurrencyOfPreference  string            `json:"currencyOfPreference,omitempty"`
	LanguagesOfPreference []string          `json:"languagesOfPreference,omitempty"`
	Properties            map[string]string `json:"properties,omitempty"`
	Resources             []Resource        `json:"resources,omitempty"`
	VariationCount        int               `json:"variationCount,omitempty"`
	VariationPage         int               `json:"variationPage,omitempty"`
}

type GetBrowseNodesRequest struct {
	BrowseNodeIDs         []string   `json:"browseNodeIds"`
	LanguagesOfPreference []string   `json:"languagesOfPreference,omitempty"`
	Resources             []Resource `json:"resources,omitempty"`
}

// CatalogWorkflow exposes the bounded Amazon Creators API Catalog v1 surface.
type CatalogWorkflow interface {
	SearchItems(context.Context, SearchItemsRequest, ...socialhub.CallOption) (SearchItemsResponse, error)
	GetItems(context.Context, GetItemsRequest, ...socialhub.CallOption) (GetItemsResponse, error)
	GetVariations(context.Context, GetVariationsRequest, ...socialhub.CallOption) (GetVariationsResponse, error)
	GetBrowseNodes(context.Context, GetBrowseNodesRequest, ...socialhub.CallOption) (GetBrowseNodesResponse, error)
}

type catalogOperation uint8

const (
	operationSearchItems catalogOperation = iota + 1
	operationGetItems
	operationGetVariations
	operationGetBrowseNodes
)

// ExactValue preserves a provider scalar without float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) || trimmed[0] == '{' || trimmed[0] == '[' {
		return fmt.Errorf("amazoncreators: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("amazoncreators: exact value must be a JSON string, number, or null")
	}
	value.raw = append(value.raw[:0], trimmed...)
	return nil
}

func (value ExactValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value ExactValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value ExactValue) IsSet() bool   { return len(value.raw) > 0 }
func (value ExactValue) IsNull() bool  { return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null")) }

func (value ExactValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value ExactValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("amazoncreators: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ProviderError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Money struct {
	Amount        ExactValue `json:"amount"`
	Currency      string     `json:"currency"`
	DisplayAmount string     `json:"displayAmount"`
}

type ImageSize struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type ImageType struct {
	Small          *ImageSize `json:"small"`
	Medium         *ImageSize `json:"medium"`
	Large          *ImageSize `json:"large"`
	HighResolution *ImageSize `json:"hiRes"`
}

type Images struct {
	Primary  *ImageType  `json:"primary"`
	Variants []ImageType `json:"variants"`
}

type StringAttribute struct {
	DisplayValue string `json:"displayValue"`
	Label        string `json:"label"`
	Locale       string `json:"locale"`
}

type MultiValuedAttribute struct {
	DisplayValues []string `json:"displayValues"`
	Label         string   `json:"label"`
	Locale        string   `json:"locale"`
}

type ItemInfo struct {
	ByLineInfo      json.RawMessage       `json:"byLineInfo"`
	Classifications json.RawMessage       `json:"classifications"`
	ContentInfo     json.RawMessage       `json:"contentInfo"`
	ContentRating   json.RawMessage       `json:"contentRating"`
	ExternalIDs     json.RawMessage       `json:"externalIds"`
	Features        *MultiValuedAttribute `json:"features"`
	ManufactureInfo json.RawMessage       `json:"manufactureInfo"`
	ProductInfo     json.RawMessage       `json:"productInfo"`
	TechnicalInfo   json.RawMessage       `json:"technicalInfo"`
	Title           *StringAttribute      `json:"title"`
	TradeInInfo     json.RawMessage       `json:"tradeInInfo"`
	Raw             json.RawMessage       `json:"-"`
}

func (value *ItemInfo) UnmarshalJSON(data []byte) error {
	type wire ItemInfo
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ItemInfo(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type OfferAvailability struct {
	Message          string `json:"message"`
	MaxOrderQuantity int    `json:"maxOrderQuantity"`
	MinOrderQuantity int    `json:"minOrderQuantity"`
	Type             string `json:"type"`
}

type OfferCondition struct {
	Value         string `json:"value"`
	SubCondition  string `json:"subCondition"`
	ConditionNote string `json:"conditionNote"`
}

type OfferMerchantInfo struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type OfferSavings struct {
	Money      *Money     `json:"money"`
	Percentage ExactValue `json:"percentage"`
}

type OfferSavingBasis struct {
	Money                *Money `json:"money"`
	SavingBasisType      string `json:"savingBasisType"`
	SavingBasisTypeLabel string `json:"savingBasisTypeLabel"`
}

type OfferPrice struct {
	Money        *Money            `json:"money"`
	PricePerUnit *Money            `json:"pricePerUnit"`
	Savings      *OfferSavings     `json:"savings"`
	SavingBasis  *OfferSavingBasis `json:"savingBasis"`
}

type OfferListing struct {
	Availability   *OfferAvailability `json:"availability"`
	Condition      *OfferCondition    `json:"condition"`
	DealDetails    json.RawMessage    `json:"dealDetails"`
	IsBuyBoxWinner bool               `json:"isBuyBoxWinner"`
	LoyaltyPoints  json.RawMessage    `json:"loyaltyPoints"`
	MerchantInfo   *OfferMerchantInfo `json:"merchantInfo"`
	Price          *OfferPrice        `json:"price"`
	Type           string             `json:"type"`
	ViolatesMAP    bool               `json:"violatesMAP"`
	Raw            json.RawMessage    `json:"-"`
}

func (value *OfferListing) UnmarshalJSON(data []byte) error {
	type wire OfferListing
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = OfferListing(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type OffersV2 struct {
	Listings []OfferListing `json:"listings"`
}

type VariationAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CustomerReviews struct {
	Count      int64   `json:"count"`
	StarRating *Rating `json:"starRating"`
}

type Rating struct {
	Value ExactValue `json:"value"`
}

type Item struct {
	ASIN                string               `json:"asin"`
	ParentASIN          string               `json:"parentASIN"`
	DetailPageURL       string               `json:"detailPageURL"`
	Images              *Images              `json:"images"`
	ItemInfo            *ItemInfo            `json:"itemInfo"`
	OffersV2            *OffersV2            `json:"offersV2"`
	BrowseNodeInfo      json.RawMessage      `json:"browseNodeInfo"`
	CustomerReviews     *CustomerReviews     `json:"customerReviews"`
	Score               ExactValue           `json:"score"`
	VariationAttributes []VariationAttribute `json:"variationAttributes"`
	Raw                 json.RawMessage      `json:"-"`
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

type SearchResult struct {
	TotalResultCount  int64           `json:"totalResultCount"`
	SearchURL         string          `json:"searchURL"`
	Items             []Item          `json:"items"`
	SearchRefinements json.RawMessage `json:"searchRefinements"`
	Raw               json.RawMessage `json:"-"`
}

func (value *SearchResult) UnmarshalJSON(data []byte) error {
	type wire SearchResult
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchResult(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type VariationSummaryPrice struct {
	HighestPrice *Money `json:"highestPrice"`
	LowestPrice  *Money `json:"lowestPrice"`
}

type VariationDimension struct {
	DisplayName string   `json:"displayName"`
	Locale      string   `json:"locale"`
	Name        string   `json:"name"`
	Values      []string `json:"values"`
}

type VariationSummary struct {
	PageCount           int                    `json:"pageCount"`
	Price               *VariationSummaryPrice `json:"price"`
	VariationCount      int                    `json:"variationCount"`
	VariationDimensions []VariationDimension   `json:"variationDimensions"`
}

type VariationsResult struct {
	Items            []Item            `json:"items"`
	VariationSummary *VariationSummary `json:"variationSummary"`
	Raw              json.RawMessage   `json:"-"`
}

func (value *VariationsResult) UnmarshalJSON(data []byte) error {
	type wire VariationsResult
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = VariationsResult(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type BrowseNodeAncestor struct {
	Ancestor        *BrowseNodeAncestor `json:"ancestor"`
	ContextFreeName string              `json:"contextFreeName"`
	DisplayName     string              `json:"displayName"`
	ID              string              `json:"id"`
}

type BrowseNodeChild struct {
	ContextFreeName string `json:"contextFreeName"`
	DisplayName     string `json:"displayName"`
	ID              string `json:"id"`
}

type BrowseNode struct {
	Ancestor        *BrowseNodeAncestor `json:"ancestor"`
	Children        []BrowseNodeChild   `json:"children"`
	ContextFreeName string              `json:"contextFreeName"`
	DisplayName     string              `json:"displayName"`
	ID              string              `json:"id"`
	IsRoot          bool                `json:"isRoot"`
	SalesRank       int64               `json:"salesRank"`
	Raw             json.RawMessage     `json:"-"`
}

func (value *BrowseNode) UnmarshalJSON(data []byte) error {
	type wire BrowseNode
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = BrowseNode(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type BrowseNodesResult struct {
	BrowseNodes []BrowseNode    `json:"browseNodes"`
	Raw         json.RawMessage `json:"-"`
}

func (value *BrowseNodesResult) UnmarshalJSON(data []byte) error {
	type wire BrowseNodesResult
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = BrowseNodesResult(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type SearchItemsResponse struct {
	SearchResult *SearchResult   `json:"searchResult"`
	Errors       []ProviderError `json:"errors"`
	Meta         ResponseMeta    `json:"-"`
	Raw          json.RawMessage `json:"-"`
}

func (value *SearchItemsResponse) UnmarshalJSON(data []byte) error {
	type wire SearchItemsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = SearchItemsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type GetItemsResponse struct {
	ItemsResult *ItemsResult    `json:"itemsResult"`
	Errors      []ProviderError `json:"errors"`
	Meta        ResponseMeta    `json:"-"`
	Raw         json.RawMessage `json:"-"`
}

type ItemsResult struct {
	Items []Item          `json:"items"`
	Raw   json.RawMessage `json:"-"`
}

func (value *ItemsResult) UnmarshalJSON(data []byte) error {
	type wire ItemsResult
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ItemsResult(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func (value *GetItemsResponse) UnmarshalJSON(data []byte) error {
	type wire GetItemsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = GetItemsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type GetVariationsResponse struct {
	VariationsResult *VariationsResult `json:"variationsResult"`
	Errors           []ProviderError   `json:"errors"`
	Meta             ResponseMeta      `json:"-"`
	Raw              json.RawMessage   `json:"-"`
}

func (value *GetVariationsResponse) UnmarshalJSON(data []byte) error {
	type wire GetVariationsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = GetVariationsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type GetBrowseNodesResponse struct {
	BrowseNodesResult *BrowseNodesResult `json:"browseNodesResult"`
	Errors            []ProviderError    `json:"errors"`
	Meta              ResponseMeta       `json:"-"`
	Raw               json.RawMessage    `json:"-"`
}

func (value *GetBrowseNodesResponse) UnmarshalJSON(data []byte) error {
	type wire GetBrowseNodesResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = GetBrowseNodesResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("amazoncreators: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
