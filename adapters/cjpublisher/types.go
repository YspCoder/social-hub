package cjpublisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxExactValueBytes     = 256
	maxProviderObjectBytes = 8 << 20
)

// ExactValue preserves provider decimals, identifiers, and counters without
// float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("cjpublisher: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("cjpublisher: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("cjpublisher: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID string
}

type PartnerStatus string

const (
	PartnerJoined    PartnerStatus = "JOINED"
	PartnerNotJoined PartnerStatus = "NOT_JOINED"
)

type FeedType string

const (
	FeedShopping FeedType = "SHOPPING"
	FeedTravel   FeedType = "TRAVEL"
	FeedFinance  FeedType = "FINANCE"
	FeedAll      FeedType = "ALL"
)

type Availability string

const (
	AvailabilityInStock    Availability = "IN_STOCK"
	AvailabilityOutOfStock Availability = "OUT_OF_STOCK"
	AvailabilityPreorder   Availability = "PREORDER"
	AvailabilityBackorder  Availability = "BACKORDER"
)

type ProductSort string

const (
	ProductSortLastUpdated ProductSort = "LAST_UPDATED"
	ProductSortPrice       ProductSort = "PRICE"
)

type SortOrder string

const (
	SortAscending  SortOrder = "ASC"
	SortDescending SortOrder = "DESC"
)

type CommissionLockingMethod string

const (
	LockingImmediate     CommissionLockingMethod = "IMMEDIATE"
	LockingFixedDate     CommissionLockingMethod = "FIXED_DATE"
	LockingOpenEnded     CommissionLockingMethod = "OPEN_ENDED"
	LockingFixedDuration CommissionLockingMethod = "FIXED_DURATION"
)

type CommissionValidationStatus string

const (
	ValidationPending   CommissionValidationStatus = "PENDING"
	ValidationAccepted  CommissionValidationStatus = "ACCEPTED"
	ValidationDeclined  CommissionValidationStatus = "DECLINED"
	ValidationAutomated CommissionValidationStatus = "AUTOMATED"
)

type LinkRelationship string

const (
	LinkRelationshipJoined    LinkRelationship = "joined"
	LinkRelationshipNotJoined LinkRelationship = "notjoined"
)

type SearchProductFeedsRequest struct {
	FeedType          FeedType
	PartnerIDs        []string
	AdvertiserCountry string
	Offset            int
	Limit             int
}

type SearchProductsRequest struct {
	AdIDs                   []string
	Keywords                []string
	PartnerIDs              []string
	PartnerStatus           PartnerStatus
	ExcludePartnerIDs       []string
	ProductIDs              []string
	ExcludeProductIDs       []string
	AdvertiserCountries     []string
	HighPrice               *float64
	LowPrice                *float64
	Currency                string
	ItemListIDs             []string
	IncludeDeletedProducts  *bool
	ServiceableAreas        []string
	ExcludeServiceableAreas []string
	Availability            Availability
	SortBy                  ProductSort
	SortOrder               SortOrder
	Offset                  int
	Limit                   int
	Page                    string
	IncludeLinkCode         bool
	PromotionalPropertyID   string
	ShopperID               string
}

type ListPublisherCommissionsRequest struct {
	SincePostingDate   time.Time
	BeforePostingDate  time.Time
	SinceEventDate     time.Time
	BeforeEventDate    time.Time
	SinceLockingDate   time.Time
	BeforeLockingDate  time.Time
	SinceCommissionID  string
	CommissionIDs      []string
	AdvertiserIDs      []string
	AdIDs              []string
	WebsiteIDs         []string
	ActionStatuses     []string
	ActionTypes        []string
	LockingMethods     []CommissionLockingMethod
	ValidationStatuses []CommissionValidationStatus
}

type ListProgramTermsRequest struct {
	AdvertiserID string
	ActiveAfter  time.Time
	ActiveBefore time.Time
	Offset       int
	Limit        int
}

type SearchLinksRequest struct {
	WebsiteID          string
	AdvertiserIDs      []string
	Relationship       LinkRelationship
	Keywords           string
	Categories         []string
	LinkType           string
	PromotionType      string
	PromotionStartDate time.Time
	PromotionEndDate   time.Time
	OngoingPromotion   bool
	PageNumber         int
	RecordsPerPage     int
	Language           string
	AllowDeepLinking   *bool
	EventName          string
	LinkID             string
	LastUpdated        time.Time
	CrossDeviceOnly    *bool
	MobileAppDownload  *bool
	MobileOptimized    *bool
	TargetedCountry    string
}

// PublisherWorkflow is the bounded current CJ publisher surface implemented
// by this adapter.
type PublisherWorkflow interface {
	SearchProductFeeds(context.Context, SearchProductFeedsRequest, ...socialhub.CallOption) (ProductFeedsResponse, error)
	SearchProducts(context.Context, SearchProductsRequest, ...socialhub.CallOption) (ProductsResponse, error)
	ListPublisherCommissions(context.Context, ListPublisherCommissionsRequest, ...socialhub.CallOption) (CommissionsResponse, error)
	ListProgramTerms(context.Context, ListProgramTermsRequest, ...socialhub.CallOption) (ProgramTermsResponse, error)
	SearchLinks(context.Context, SearchLinksRequest, ...socialhub.CallOption) (LinksResponse, error)
}

type Amount struct {
	Amount   ExactValue `json:"amount"`
	Currency string     `json:"currency"`
}

type ProductLinkCode struct {
	HTML     string `json:"html"`
	ClickURL string `json:"clickUrl"`
	ImageURL string `json:"imageUrl"`
}

type ProductShipping struct {
	Price               Amount `json:"price"`
	Service             string `json:"service"`
	LocationGroupName   string `json:"locationGroupName"`
	Region              string `json:"region"`
	PostalCode          string `json:"postalCode"`
	LocationID          string `json:"locationId"`
	Country             string `json:"country"`
	MinimumHandlingTime string `json:"minimumHandlingTime"`
	MaximumHandlingTime string `json:"maximumHandlingTime"`
	MinimumTransitTime  string `json:"minimumTransitTime"`
	MaximumTransitTime  string `json:"maximumTransitTime"`
}

type Product struct {
	ID                          string           `json:"id"`
	AdID                        string           `json:"adId"`
	AdvertiserID                string           `json:"advertiserId"`
	AdvertiserName              string           `json:"advertiserName"`
	AdvertiserCountry           string           `json:"advertiserCountry"`
	Brand                       string           `json:"brand"`
	CatalogID                   string           `json:"catalogId"`
	Description                 string           `json:"description"`
	Title                       string           `json:"title"`
	ImageLink                   string           `json:"imageLink"`
	AdditionalImageLinks        []string         `json:"additionalImageLink"`
	Deleted                     bool             `json:"isDeleted"`
	ItemListID                  string           `json:"itemListId"`
	ItemListName                string           `json:"itemListName"`
	LastUpdated                 string           `json:"lastUpdated"`
	Link                        string           `json:"link"`
	MobileLink                  string           `json:"mobileLink"`
	LinkCode                    *ProductLinkCode `json:"linkCode"`
	Price                       Amount           `json:"price"`
	SalePrice                   *Amount          `json:"salePrice"`
	SalePriceEffectiveDateStart string           `json:"salePriceEffectiveDateStart"`
	SalePriceEffectiveDateEnd   string           `json:"salePriceEffectiveDateEnd"`
	ServiceableAreas            string           `json:"serviceableAreas"`
	Shipping                    *ProductShipping `json:"shipping"`
	SourceFeedType              string           `json:"sourceFeedType"`
	TargetCountry               string           `json:"targetCountry"`
	Raw                         json.RawMessage  `json:"-"`
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

type ProductFeed struct {
	AdID              string          `json:"adId"`
	AdvertiserID      string          `json:"advertiserId"`
	AdvertiserName    string          `json:"advertiserName"`
	AdvertiserCountry string          `json:"advertiserCountry"`
	SourceFeedType    string          `json:"sourceFeedType"`
	Currency          string          `json:"currency"`
	Language          string          `json:"language"`
	FeedName          string          `json:"feedName"`
	LastUpdated       string          `json:"lastUpdated"`
	ProductCount      ExactValue      `json:"productCount"`
	Raw               json.RawMessage `json:"-"`
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
	Feeds      []ProductFeed   `json:"resultList"`
	TotalCount ExactValue      `json:"totalCount"`
	Limit      ExactValue      `json:"limit"`
	Count      ExactValue      `json:"count"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

type ProductsResponse struct {
	Products   []Product       `json:"resultList"`
	TotalCount ExactValue      `json:"totalCount"`
	Limit      ExactValue      `json:"limit"`
	Count      ExactValue      `json:"count"`
	NextPage   string          `json:"nextPage"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

type CommissionItem struct {
	CommissionItemID                 string          `json:"commissionItemId"`
	DiscountUSD                      ExactValue      `json:"discountUsd"`
	DiscountPublisherCurrency        ExactValue      `json:"discountPubCurrency"`
	ItemListID                       string          `json:"itemListId"`
	TotalCommissionPublisherCurrency ExactValue      `json:"totalCommissionPubCurrency"`
	TotalCommissionUSD               ExactValue      `json:"totalCommissionUsd"`
	Quantity                         ExactValue      `json:"quantity"`
	PerItemSalePublisherCurrency     ExactValue      `json:"perItemSaleAmountPubCurrency"`
	PerItemSaleUSD                   ExactValue      `json:"perItemSaleAmountUsd"`
	SKU                              string          `json:"sku"`
	Raw                              json.RawMessage `json:"-"`
}

func (value *CommissionItem) UnmarshalJSON(data []byte) error {
	type wire CommissionItem
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CommissionItem(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Commission struct {
	ActionStatus                string           `json:"actionStatus"`
	ActionTrackerID             string           `json:"actionTrackerId"`
	ActionTrackerName           string           `json:"actionTrackerName"`
	ActionType                  string           `json:"actionType"`
	AdvertiserID                string           `json:"advertiserId"`
	AdvertiserName              string           `json:"advertiserName"`
	AdID                        string           `json:"aid"`
	ClickDate                   string           `json:"clickDate"`
	CommissionID                string           `json:"commissionId"`
	CorrectionReason            string           `json:"correctionReason"`
	Country                     string           `json:"country"`
	Coupon                      string           `json:"coupon"`
	EventDate                   string           `json:"eventDate"`
	CrossDevice                 *bool            `json:"isCrossDevice"`
	LockingDate                 string           `json:"lockingDate"`
	LockingMethod               string           `json:"lockingMethod"`
	OrderID                     string           `json:"orderId"`
	ValidationStatus            string           `json:"validationStatus"`
	Original                    bool             `json:"original"`
	OriginalActionID            string           `json:"originalActionId"`
	PostingDate                 string           `json:"postingDate"`
	PublisherCommissionCurrency ExactValue       `json:"pubCommissionAmountPubCurrency"`
	PublisherCommissionUSD      ExactValue       `json:"pubCommissionAmountUsd"`
	PublisherID                 string           `json:"publisherId"`
	PublisherName               string           `json:"publisherName"`
	SaleAmountPublisherCurrency ExactValue       `json:"saleAmountPubCurrency"`
	SaleAmountUSD               ExactValue       `json:"saleAmountUsd"`
	Source                      string           `json:"source"`
	WebsiteID                   string           `json:"websiteId"`
	WebsiteName                 string           `json:"websiteName"`
	ShopperID                   string           `json:"shopperId"`
	Items                       []CommissionItem `json:"items"`
	Raw                         json.RawMessage  `json:"-"`
}

func (value *Commission) UnmarshalJSON(data []byte) error {
	type wire Commission
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Commission(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CommissionsResponse struct {
	Count           ExactValue      `json:"count"`
	Limit           ExactValue      `json:"limit"`
	MaxCommissionID string          `json:"maxCommissionId"`
	PayloadComplete bool            `json:"payloadComplete"`
	Commissions     []Commission    `json:"records"`
	Meta            ResponseMeta    `json:"-"`
	Raw             json.RawMessage `json:"-"`
}

type NamedTerm struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CommissionRate struct {
	Type     string     `json:"type"`
	Value    ExactValue `json:"value"`
	Currency string     `json:"currency"`
}

type ProgramCommission struct {
	Rank                  ExactValue     `json:"rank"`
	Situation             *NamedTerm     `json:"situation"`
	ItemList              *NamedTerm     `json:"itemList"`
	PromotionalProperties []NamedTerm    `json:"promotionalProperties"`
	ViewThrough           bool           `json:"isViewThrough"`
	Rate                  CommissionRate `json:"rate"`
}

type PerformanceIncentiveValue struct {
	Type           string     `json:"type"`
	CommissionType string     `json:"commissionType"`
	Value          ExactValue `json:"value"`
}

type PerformanceIncentive struct {
	Threshold PerformanceIncentiveValue `json:"threshold"`
	Reward    PerformanceIncentiveValue `json:"reward"`
	Currency  string                    `json:"currency"`
}

type ActionTracker struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type ProgramActionTerm struct {
	ID                    string                 `json:"id"`
	ActionTracker         ActionTracker          `json:"actionTracker"`
	ReferralPeriod        ExactValue             `json:"referralPeriod"`
	ReferralOccurrences   ExactValue             `json:"referralOccurrences"`
	LockingMethod         ProgramLockingMethod   `json:"lockingMethod"`
	PerformanceIncentives []PerformanceIncentive `json:"performanceIncentives"`
	Commissions           []ProgramCommission    `json:"commissions"`
}

type ProgramLockingMethod struct {
	Type           string     `json:"type"`
	DurationInDays ExactValue `json:"durationInDays"`
}

type SpecialTerms struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

type ProgramTerms struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	SpecialTerms *SpecialTerms       `json:"specialTerms"`
	Default      bool                `json:"isDefault"`
	ActionTerms  []ProgramActionTerm `json:"actionTerms"`
	Raw          json.RawMessage     `json:"-"`
}

func (value *ProgramTerms) UnmarshalJSON(data []byte) error {
	type wire ProgramTerms
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ProgramTerms(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProgramContract struct {
	StartTime    string          `json:"startTime"`
	EndTime      string          `json:"endTime"`
	Status       string          `json:"status"`
	AdvertiserID string          `json:"advertiserId"`
	ProgramTerms ProgramTerms    `json:"programTerms"`
	Raw          json.RawMessage `json:"-"`
}

func (value *ProgramContract) UnmarshalJSON(data []byte) error {
	type wire ProgramContract
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ProgramContract(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProgramTermsResponse struct {
	TotalCount ExactValue        `json:"totalCount"`
	Count      ExactValue        `json:"count"`
	Contracts  []ProgramContract `json:"resultList"`
	Meta       ResponseMeta      `json:"-"`
	Raw        json.RawMessage   `json:"-"`
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || int64(len(trimmed)) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("cjpublisher: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

var _ PublisherWorkflow = (*Client)(nil)
