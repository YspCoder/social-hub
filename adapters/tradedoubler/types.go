package tradedoubler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	maxExactValueBytes     = 256
	maxRawValueBytes       = 1 << 20
	maxProviderObjectBytes = 8 << 20
)

type ResponseMeta struct {
	RequestID          string
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
}

// ExactValue preserves a provider JSON string, number, or null without
// float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("tradedoubler: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("tradedoubler: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("tradedoubler: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

// RawValue preserves one provider JSON value of any type.
type RawValue struct {
	raw json.RawMessage
}

func (value *RawValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxRawValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("tradedoubler: invalid raw value")
	}
	value.raw = append(value.raw[:0], trimmed...)
	return nil
}

func (value RawValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value RawValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value RawValue) IsSet() bool   { return len(value.raw) > 0 }
func (value RawValue) IsNull() bool  { return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null")) }

func (value RawValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value RawValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("tradedoubler: decode target and raw value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ProductOrder string

type DateOutputFormat string

const (
	OrderPriceAscending         ProductOrder     = "priceAsc"
	OrderPriceDescending        ProductOrder     = "priceDesc"
	OrderModificationAscending  ProductOrder     = "modificationDateAsc"
	OrderModificationDescending ProductOrder     = "modificationDateDesc"
	DateOutputISO8601           DateOutputFormat = "iso8601"
	MaximumSearchProducts                        = 1000
)

type SearchProductsRequest struct {
	FeedIDs                 []int64
	Keyword                 string
	Currency                string
	IncludeSourceProductURL *bool
	MinPrice                string
	MaxPrice                string
	MinUpdateDate           string
	MaxUpdateDate           string
	TDCategoryIDs           []int64
	Brands                  []string
	Language                string
	OrderBy                 ProductOrder
	Page                    int
	PageSize                int
	Limit                   int
	GroupOffersByProduct    *bool
	IncludePriceHistory     *bool
	DateOutputFormat        DateOutputFormat
}

type ListProductFeedsRequest struct {
	ProgramIDs []int64
}

type GetUnlimitedFeedLastUpdatedRequest struct {
	FeedID int64
}

// ProductsWorkflow exposes the bounded Tradedoubler Products API surface.
type ProductsWorkflow interface {
	SearchProducts(context.Context, SearchProductsRequest, ...socialhub.CallOption) (ProductsResponse, error)
	ListProductFeeds(context.Context, ListProductFeedsRequest, ...socialhub.CallOption) (ProductFeedsResponse, error)
	GetUnlimitedFeedLastUpdated(context.Context, GetUnlimitedFeedLastUpdatedRequest, ...socialhub.CallOption) (UnlimitedFeedLastUpdatedResponse, error)
}

type ProductHeader struct {
	TotalHits ExactValue `json:"totalHits"`
}

type ProductImage struct {
	URL    string     `json:"url"`
	Width  ExactValue `json:"width"`
	Height ExactValue `json:"height"`
}

type ProductIdentifiers struct {
	EAN  ExactValue `json:"ean"`
	SKU  ExactValue `json:"sku"`
	UPC  ExactValue `json:"upc"`
	ISBN ExactValue `json:"isbn"`
	MPN  ExactValue `json:"mpn"`
}

type ProductField struct {
	Name  string   `json:"name"`
	Value RawValue `json:"value"`
}

type ProductCategory struct {
	ID             ExactValue `json:"id"`
	Name           string     `json:"name"`
	TDCategoryName string     `json:"tdCategoryName"`
}

type Price struct {
	Value    ExactValue `json:"value"`
	Currency string     `json:"currency"`
}

type PriceHistoryEntry struct {
	Price Price      `json:"price"`
	Date  ExactValue `json:"date"`
}

type Offer struct {
	FeedID           ExactValue          `json:"feedId"`
	ProductURL       string              `json:"productUrl"`
	PriceHistory     []PriceHistoryEntry `json:"priceHistory"`
	Modified         ExactValue          `json:"modified"`
	Availability     string              `json:"availability"`
	DeliveryTime     string              `json:"deliveryTime"`
	Condition        string              `json:"condition"`
	ShippingCost     string              `json:"shippingCost"`
	SourceProductID  string              `json:"sourceProductId"`
	ProgramName      string              `json:"programName"`
	ID               string              `json:"id"`
	SourceProductURL string              `json:"sourceProductUrl"`
	Price            Price               `json:"price"`
	GroupingID       string              `json:"groupingId"`
	InStock          ExactValue          `json:"inStock"`
	Raw              json.RawMessage     `json:"-"`
}

func (value *Offer) UnmarshalJSON(data []byte) error {
	type wire Offer
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Offer(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Product struct {
	Name             string             `json:"name"`
	Language         string             `json:"language"`
	Description      string             `json:"description"`
	ShortDescription string             `json:"shortDescription"`
	ProductImage     ProductImage       `json:"productImage"`
	Identifiers      ProductIdentifiers `json:"identifiers"`
	Fields           []ProductField     `json:"fields"`
	Offers           []Offer            `json:"offers"`
	Categories       []ProductCategory  `json:"categories"`
	Brand            string             `json:"brand"`
	Manufacturer     string             `json:"manufacturer"`
	Model            string             `json:"model"`
	ProgramLogo      string             `json:"programLogo"`
	PromoText        string             `json:"promoText"`
	Size             string             `json:"size"`
	TechSpecs        string             `json:"techSpecs"`
	Warranty         string             `json:"warranty"`
	Weight           string             `json:"weight"`
	Raw              json.RawMessage    `json:"-"`
}

func (value *Product) UnmarshalJSON(data []byte) error {
	type wire Product
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Product(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProductsResponse struct {
	Header   ProductHeader   `json:"productHeader"`
	Products []Product       `json:"products"`
	Meta     ResponseMeta    `json:"-"`
	Raw      json.RawMessage `json:"-"`
}

func (value *ProductsResponse) UnmarshalJSON(data []byte) error {
	type wire ProductsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ProductsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProgramSummary struct {
	ProgramID ExactValue `json:"programId"`
	Name      string     `json:"name"`
}

type ProductFeed struct {
	FeedID                     ExactValue       `json:"feedId"`
	Name                       string           `json:"name"`
	Active                     bool             `json:"active"`
	SendToNewProductFeed       bool             `json:"sendToNewPF"`
	Visible                    bool             `json:"visible"`
	CurrencyISOCode            string           `json:"currencyISOCode"`
	LanguageISOCode            string           `json:"languageISOCode"`
	Secret                     bool             `json:"secret"`
	NumberOfUnmappedCategories ExactValue       `json:"numberOfUnmappedCategories"`
	NumberOfProducts           ExactValue       `json:"numberOfProducts"`
	LastModifiedTime           string           `json:"lastModifiedTime"`
	Programs                   []ProgramSummary `json:"programs"`
	Raw                        json.RawMessage  `json:"-"`
}

func (value *ProductFeed) UnmarshalJSON(data []byte) error {
	type wire ProductFeed
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ProductFeed(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProductFeedsResponse struct {
	Feeds []ProductFeed   `json:"feeds"`
	Meta  ResponseMeta    `json:"-"`
	Raw   json.RawMessage `json:"-"`
}

func (value *ProductFeedsResponse) UnmarshalJSON(data []byte) error {
	type wire ProductFeedsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ProductFeedsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type UnlimitedFeedLastUpdatedResponse struct {
	FeedIDs         []ExactValue    `json:"feedIds"`
	LastUpdatedTime string          `json:"lastUpdatedTime"`
	Meta            ResponseMeta    `json:"-"`
	Raw             json.RawMessage `json:"-"`
}

func (value *UnlimitedFeedLastUpdatedResponse) UnmarshalJSON(data []byte) error {
	type wire UnlimitedFeedLastUpdatedResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = UnlimitedFeedLastUpdatedResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("tradedoubler: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
