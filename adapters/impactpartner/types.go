package impactpartner

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

type ResponseMeta struct {
	RequestID              string
	RateLimitLimitHour     string
	RateLimitRemainingHour string
	RateLimitReset         string
}

type PageMetadata struct {
	Page            ExactValue `json:"@page"`
	NumberOfPages   ExactValue `json:"@numpages"`
	PageSize        ExactValue `json:"@pagesize"`
	Total           ExactValue `json:"@total"`
	Start           ExactValue `json:"@start"`
	End             ExactValue `json:"@end"`
	URI             string     `json:"@uri"`
	FirstPageURI    string     `json:"@firstpageuri"`
	PreviousPageURI string     `json:"@previouspageuri"`
	NextPageURI     string     `json:"@nextpageuri"`
	LastPageURI     string     `json:"@lastpageuri"`
}

// ExactValue preserves a provider JSON string, number, or null without
// float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("impactpartner: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("impactpartner: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("impactpartner: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type InsertionOrderStatus string

const (
	InsertionOrderActive  InsertionOrderStatus = "Active"
	InsertionOrderExpired InsertionOrderStatus = "Expired"
)

type TrackingLinkType string

const (
	TrackingLinkRegular TrackingLinkType = "Regular"
	TrackingLinkVanity  TrackingLinkType = "Vanity"
)

type ActionState string

const (
	ActionPending  ActionState = "PENDING"
	ActionApproved ActionState = "APPROVED"
	ActionReversed ActionState = "REVERSED"
)

type ListProgramsRequest struct {
	InsertionOrderStatus InsertionOrderStatus
}

type SearchCatalogItemsRequest struct {
	Keyword  string
	PageSize int
	Page     int
}

type GetCatalogItemRequest struct {
	CatalogID string
	ItemID    string
}

type CreateTrackingLinkRequest struct {
	ProgramID              string
	Type                   TrackingLinkType
	CustomPath             string
	AdID                   string
	DeepLink               string
	MediaPartnerPropertyID string
	SubID1                 string
	SubID2                 string
	SubID3                 string
	SharedID               string
}

type ListActionsRequest struct {
	CampaignID       int64
	State            ActionState
	ActionDateStart  time.Time
	ActionDateEnd    time.Time
	StartDate        time.Time
	EndDate          time.Time
	LockingDateStart time.Time
	LockingDateEnd   time.Time
	Page             int
	PageSize         int
}

// PartnerWorkflow exposes the bounded impact.com Partner API v16 surface.
type PartnerWorkflow interface {
	ListPrograms(context.Context, ListProgramsRequest, ...socialhub.CallOption) (ProgramsResponse, error)
	SearchCatalogItems(context.Context, SearchCatalogItemsRequest, ...socialhub.CallOption) (CatalogItemsResponse, error)
	GetCatalogItem(context.Context, GetCatalogItemRequest, ...socialhub.CallOption) (CatalogItem, error)
	CreateTrackingLink(context.Context, CreateTrackingLinkRequest, ...socialhub.CallOption) (TrackingLink, error)
	ListActions(context.Context, ListActionsRequest, ...socialhub.CallOption) (ActionsResponse, error)
}

type Program struct {
	AdvertiserID        string          `json:"AdvertiserId"`
	AdvertiserName      string          `json:"AdvertiserName"`
	AdvertiserURL       string          `json:"AdvertiserUrl"`
	CampaignID          string          `json:"CampaignId"`
	CampaignName        string          `json:"CampaignName"`
	CampaignURL         string          `json:"CampaignUrl"`
	CampaignDescription string          `json:"CampaignDescription"`
	Type                string          `json:"Type"`
	ShippingRegions     []string        `json:"ShippingRegions"`
	CampaignLogoURI     string          `json:"CampaignLogoUri"`
	PublicTermsURI      string          `json:"PublicTermsUri"`
	ContractStatus      string          `json:"ContractStatus"`
	ContractURI         string          `json:"ContractUri"`
	TrackingLink        string          `json:"TrackingLink"`
	HasStandDownPolicy  string          `json:"HasStandDownPolicy"`
	AllowsDeepLinking   string          `json:"AllowsDeeplinking"`
	DeepLinkDomains     []string        `json:"DeeplinkDomains"`
	URI                 string          `json:"Uri"`
	Raw                 json.RawMessage `json:"-"`
}

func (value *Program) UnmarshalJSON(data []byte) error {
	type wire Program
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Program(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProgramsResponse struct {
	Programs []Program `json:"Campaigns"`
	PageMetadata
	Meta ResponseMeta    `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

func (value *ProgramsResponse) UnmarshalJSON(data []byte) error {
	type wire ProgramsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ProgramsResponse(decoded)
	applyPaginationAliases(data, &value.PageMetadata)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Promotion struct {
	AdvertiserID            string `json:"AdvertiserId"`
	AdvertiserName          string `json:"AdvertiserName"`
	PromotionFileID         string `json:"PromotionFileId"`
	PromotionIDs            string `json:"PromotionIds"`
	PromotionTitle          string `json:"PromotionTitle"`
	GenericRedemptionCode   string `json:"GenericRedemptionCode"`
	PromotionEffectiveDates string `json:"PromotionEffectiveDates"`
	URI                     string `json:"Uri"`
}

type CatalogItem struct {
	ID                         string          `json:"Id"`
	CatalogID                  string          `json:"CatalogId"`
	CampaignID                 string          `json:"CampaignId"`
	CampaignName               string          `json:"CampaignName"`
	CatalogItemID              string          `json:"CatalogItemId"`
	Name                       string          `json:"Name"`
	Description                string          `json:"Description"`
	MultiPack                  string          `json:"MultiPack"`
	Bullets                    []string        `json:"Bullets"`
	Labels                     []string        `json:"Labels"`
	Manufacturer               string          `json:"Manufacturer"`
	URL                        string          `json:"Url"`
	MobileURL                  string          `json:"MobileUrl"`
	ImageURL                   string          `json:"ImageUrl"`
	ProductBid                 string          `json:"ProductBid"`
	AdditionalImageURLs        []string        `json:"AdditionalImageUrls"`
	Promotions                 []Promotion     `json:"Promotions"`
	CurrentPrice               string          `json:"CurrentPrice"`
	OriginalPrice              string          `json:"OriginalPrice"`
	DiscountPercentage         string          `json:"DiscountPercentage"`
	Currency                   string          `json:"Currency"`
	StockAvailability          string          `json:"StockAvailability"`
	EstimatedShipDate          string          `json:"EstimatedShipDate"`
	LaunchDate                 string          `json:"LaunchDate"`
	ExpirationDate             string          `json:"ExpirationDate"`
	GTIN                       string          `json:"Gtin"`
	GTINType                   string          `json:"GtinType"`
	ASIN                       string          `json:"Asin"`
	MPN                        string          `json:"Mpn"`
	ShippingRate               string          `json:"ShippingRate"`
	ShippingWeight             string          `json:"ShippingWeight"`
	ShippingWeightUnit         string          `json:"ShippingWeightUnit"`
	ShippingLength             string          `json:"ShippingLength"`
	ShippingWidth              string          `json:"ShippingWidth"`
	ShippingHeight             string          `json:"ShippingHeight"`
	ShippingLengthUnit         string          `json:"ShippingLengthUnit"`
	ShippingLabel              string          `json:"ShippingLabel"`
	Category                   string          `json:"Category"`
	SubCategory                string          `json:"SubCategory"`
	AdvertiserFormatCategories []string        `json:"AdvertiserFormatCategories"`
	OriginalFormatCategory     string          `json:"OriginalFormatCategory"`
	OriginalFormatCategoryID   string          `json:"OriginalFormatCategoryId"`
	ParentName                 string          `json:"ParentName"`
	ParentSKU                  string          `json:"ParentSku"`
	IsParent                   bool            `json:"IsParent"`
	ItemGroupID                string          `json:"ItemGroupId"`
	Colors                     []string        `json:"Colors"`
	Material                   string          `json:"Material"`
	Pattern                    string          `json:"Pattern"`
	Size                       string          `json:"Size"`
	SizeUnit                   string          `json:"SizeUnit"`
	Weight                     string          `json:"Weight"`
	WeightUnit                 string          `json:"WeightUnit"`
	Condition                  string          `json:"Condition"`
	AgeGroup                   string          `json:"AgeGroup"`
	AgeRangeMin                string          `json:"AgeRangeMin"`
	AgeRangeMax                string          `json:"AgeRangeMax"`
	AgeRangeUnit               string          `json:"AgeRangeUnit"`
	Gender                     string          `json:"Gender"`
	Adult                      string          `json:"Adult"`
	Text1                      string          `json:"Text1"`
	Text2                      string          `json:"Text2"`
	Text3                      string          `json:"Text3"`
	Numeric1                   string          `json:"Numeric1"`
	Numeric2                   string          `json:"Numeric2"`
	Numeric3                   string          `json:"Numeric3"`
	Money1                     string          `json:"Money1"`
	Money2                     string          `json:"Money2"`
	Money3                     string          `json:"Money3"`
	URI                        string          `json:"Uri"`
	Meta                       ResponseMeta    `json:"-"`
	Raw                        json.RawMessage `json:"-"`
}

func (value *CatalogItem) UnmarshalJSON(data []byte) error {
	type wire CatalogItem
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CatalogItem(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CatalogItemsResponse struct {
	Items []CatalogItem `json:"Items"`
	PageMetadata
	Meta ResponseMeta    `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

func (value *CatalogItemsResponse) UnmarshalJSON(data []byte) error {
	type wire CatalogItemsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CatalogItemsResponse(decoded)
	applyPaginationAliases(data, &value.PageMetadata)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type TrackingLink struct {
	TrackingURL string          `json:"TrackingURL"`
	Meta        ResponseMeta    `json:"-"`
	Raw         json.RawMessage `json:"-"`
}

func (value *TrackingLink) UnmarshalJSON(data []byte) error {
	type wire TrackingLink
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = TrackingLink(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Action struct {
	ID                string          `json:"Id"`
	CampaignID        ExactValue      `json:"CampaignId"`
	CampaignName      string          `json:"CampaignName"`
	ActionTrackerID   ExactValue      `json:"ActionTrackerId"`
	ActionTrackerName string          `json:"ActionTrackerName"`
	State             ActionState     `json:"State"`
	AdID              ExactValue      `json:"AdId"`
	Payout            string          `json:"Payout"`
	Amount            string          `json:"Amount"`
	Currency          string          `json:"Currency"`
	EventDate         string          `json:"EventDate"`
	LockingDate       string          `json:"LockingDate"`
	OrderID           string          `json:"Oid"`
	EventCode         string          `json:"EventCode"`
	SharedID          string          `json:"SharedId"`
	SubID1            string          `json:"SubId1"`
	SubID2            string          `json:"SubId2"`
	SubID3            string          `json:"SubId3"`
	DeltaPayout       string          `json:"DeltaPayout"`
	IntendedPayout    string          `json:"IntendedPayout"`
	DeltaAmount       string          `json:"DeltaAmount"`
	IntendedAmount    string          `json:"IntendedAmount"`
	ReferringDate     string          `json:"ReferringDate"`
	CreationDate      string          `json:"CreationDate"`
	ClearedDate       string          `json:"ClearedDate"`
	ReferringType     string          `json:"ReferringType"`
	ReferringDomain   string          `json:"ReferringDomain"`
	PromoCode         string          `json:"PromoCode"`
	CustomerArea      string          `json:"CustomerArea"`
	CustomerCity      string          `json:"CustomerCity"`
	CustomerRegion    string          `json:"CustomerRegion"`
	CustomerCountry   string          `json:"CustomerCountry"`
	URI               string          `json:"Uri"`
	Raw               json.RawMessage `json:"-"`
}

func (value *Action) UnmarshalJSON(data []byte) error {
	type wire Action
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Action(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ActionsResponse struct {
	Actions []Action `json:"Actions"`
	PageMetadata
	Meta ResponseMeta    `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

func (value *ActionsResponse) UnmarshalJSON(data []byte) error {
	type wire ActionsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ActionsResponse(decoded)
	applyPaginationAliases(data, &value.PageMetadata)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func applyPaginationAliases(data []byte, metadata *PageMetadata) {
	var aliases struct {
		FirstPageURI    string `json:"@firstpageruri"`
		PreviousPageURI string `json:"@previouspageruri"`
		LastPageURI     string `json:"@lastpageruri"`
	}
	if metadata == nil || json.Unmarshal(data, &aliases) != nil {
		return
	}
	if metadata.FirstPageURI == "" {
		metadata.FirstPageURI = aliases.FirstPageURI
	}
	if metadata.PreviousPageURI == "" {
		metadata.PreviousPageURI = aliases.PreviousPageURI
	}
	if metadata.LastPageURI == "" {
		metadata.LastPageURI = aliases.LastPageURI
	}
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("impactpartner: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}
