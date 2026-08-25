package mercadodisplayads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const maxExactValueBytes = 256

// ExactValue preserves a provider scalar without float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) || trimmed[0] == '{' || trimmed[0] == '[' {
		return fmt.Errorf("mercadodisplayads: invalid exact value")
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
		return fmt.Errorf("mercadodisplayads: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type SortOrder string

const (
	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

type AdvertiserSortField string

const (
	AdvertiserSortByID     AdvertiserSortField = "advertiser_id"
	AdvertiserSortBySiteID AdvertiserSortField = "site_id"
)

type ResourceSortField string

const (
	ResourceSortByID        ResourceSortField = "id"
	ResourceSortByName      ResourceSortField = "name"
	ResourceSortByStartDate ResourceSortField = "start_date"
	ResourceSortByEndDate   ResourceSortField = "end_date"
)

type CreativeSortField string

const (
	CreativeSortByID   CreativeSortField = "id"
	CreativeSortByName CreativeSortField = "name"
)

type ResponseMeta struct {
	RequestID string
}

type Advertiser struct {
	AdvertiserID   int64  `json:"advertiser_id"`
	SiteID         string `json:"site_id"`
	AdvertiserName string `json:"advertiser_name"`
	AccountName    string `json:"account_name"`
}

type AdvertiserList struct {
	Advertisers []Advertiser `json:"advertisers"`
	Meta        ResponseMeta `json:"-"`
}

type CampaignType string

const (
	CampaignTypeProgrammatic CampaignType = "PROGRAMMATIC"
	CampaignTypeGuaranteed   CampaignType = "GUARANTEED"
)

type Campaign struct {
	ID           int64        `json:"id"`
	Name         string       `json:"name"`
	StartDate    string       `json:"start_date"`
	EndDate      string       `json:"end_date"`
	AdvertiserID int64        `json:"advertiser_id"`
	Type         CampaignType `json:"type"`
	Status       string       `json:"status"`
	SiteID       string       `json:"site_id"`
	Goal         string       `json:"goal"`
}

type LineItemType string

const (
	LineItemTypeDisplay LineItemType = "Display"
	LineItemTypeSocial  LineItemType = "Social"
	LineItemTypeVideo   LineItemType = "Video"
)

type LineItem struct {
	LineItemID int64        `json:"line_item_id"`
	Name       string       `json:"name"`
	StartDate  string       `json:"start_date"`
	EndDate    string       `json:"end_date"`
	CampaignID int64        `json:"campaign_id"`
	Type       LineItemType `json:"type"`
	Status     string       `json:"status"`
}

// UnmarshalJSON accepts the documented campaign_id and the campaing_id typo
// present in Mercado Libre's current Line Item example.
func (item *LineItem) UnmarshalJSON(data []byte) error {
	var wire struct {
		LineItemID int64        `json:"line_item_id"`
		Name       string       `json:"name"`
		StartDate  string       `json:"start_date"`
		EndDate    string       `json:"end_date"`
		CampaignID *int64       `json:"campaign_id"`
		CampaingID *int64       `json:"campaing_id"`
		Type       LineItemType `json:"type"`
		Status     string       `json:"status"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.CampaignID != nil && wire.CampaingID != nil && *wire.CampaignID != *wire.CampaingID {
		return fmt.Errorf("mercadodisplayads: conflicting campaign_id and campaing_id")
	}
	campaignID := int64(0)
	if wire.CampaignID != nil {
		campaignID = *wire.CampaignID
	} else if wire.CampaingID != nil {
		campaignID = *wire.CampaingID
	}
	*item = LineItem{
		LineItemID: wire.LineItemID, Name: wire.Name, StartDate: wire.StartDate,
		EndDate: wire.EndDate, CampaignID: campaignID, Type: wire.Type, Status: wire.Status,
	}
	return nil
}

type Creative struct {
	CreativeID int64  `json:"creative_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	LineItemID int64  `json:"line_item_id"`
	CampaignID int64  `json:"campaign_id"`
}

// AttributionMetric is used by both event_time and touch_point attribution.
type AttributionMetric struct {
	CPAOrder              ExactValue `json:"cpa_order"`
	CPAPPV                ExactValue `json:"cpa_ppv"`
	ROAS                  ExactValue `json:"roas"`
	UnitsQuantity         ExactValue `json:"units_quantity"`
	DirectAmount          ExactValue `json:"direct_amount"`
	DirectItemQuantity    ExactValue `json:"direct_item_quantity"`
	AttributionPPV        ExactValue `json:"attribution_ppv"`
	AttributionAddToCart  ExactValue `json:"attribution_add_to_cart"`
	AttributionBookmark   ExactValue `json:"attribution_bookmark"`
	AttributionCheckout   ExactValue `json:"attribution_checkout"`
	AttributionLeads      ExactValue `json:"attribution_leads"`
	CostPerAttributedLead ExactValue `json:"cpl"`
}

// DeliveryMetric contains one Display Ads metric row or summary. All provider
// numeric values remain exact JSON scalars.
type DeliveryMetric struct {
	Date             Date              `json:"date"`
	SiteID           string            `json:"site_id"`
	Currency         string            `json:"currency"`
	Prints           ExactValue        `json:"prints"`
	Clicks           ExactValue        `json:"clicks"`
	ActiveViews      ExactValue        `json:"active_views"`
	CompletedViews   ExactValue        `json:"completed_views"`
	Reach            ExactValue        `json:"reach"`
	CTR              ExactValue        `json:"ctr"`
	ConsumedBudget   ExactValue        `json:"consumed_budget"`
	CPM              ExactValue        `json:"cpm"`
	CPC              ExactValue        `json:"cpc"`
	AverageFrequency ExactValue        `json:"average_frequency"`
	EventTime        AttributionMetric `json:"event_time"`
	TouchPoint       AttributionMetric `json:"touch_point"`
}

type CampaignMetrics struct {
	Metrics []DeliveryMetric `json:"metrics"`
	Summary *DeliveryMetric  `json:"summary"`
	Meta    ResponseMeta     `json:"-"`
}

type LineItemMetricGroup struct {
	CampaignID int64            `json:"campaign_id"`
	LineItemID int64            `json:"line_item_id"`
	Metrics    []DeliveryMetric `json:"metrics"`
	Summary    *DeliveryMetric  `json:"summary"`
}

type CreativeMetricGroup struct {
	CampaignID int64            `json:"campaign_id"`
	LineItemID int64            `json:"line_item_id"`
	CreativeID int64            `json:"creative_id"`
	Metrics    []DeliveryMetric `json:"metrics"`
	Summary    *DeliveryMetric  `json:"summary"`
}

type AdvertiserListRequest struct {
	SortBy    AdvertiserSortField
	SortOrder SortOrder
}

type CampaignListRequest struct {
	SortBy    ResourceSortField
	SortOrder SortOrder
}

type CampaignMetricsRequest struct {
	CampaignID int64
	DateFrom   Date
	DateTo     Date
}

type LineItemListRequest struct {
	CampaignID int64
	SortBy     ResourceSortField
	SortOrder  SortOrder
}

type LineItemMetricsRequest struct {
	DateFrom   Date
	DateTo     Date
	CampaignID int64
	IDs        []int64
}

type CreativeListRequest struct {
	CampaignID int64
	LineItemID int64
	SortBy     CreativeSortField
	SortOrder  SortOrder
}

type CreativeMetricsRequest struct {
	DateFrom   Date
	DateTo     Date
	LineItemID int64
	IDs        []int64
}

type AdvertiserWorkflow interface {
	ListAdvertisers(context.Context, AdvertiserListRequest, ...socialhub.CallOption) (AdvertiserList, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, CampaignListRequest, ...socialhub.CallOption) ([]Campaign, ResponseMeta, error)
	CampaignMetrics(context.Context, CampaignMetricsRequest, ...socialhub.CallOption) (*CampaignMetrics, error)
}

type LineItemWorkflow interface {
	ListLineItems(context.Context, LineItemListRequest, ...socialhub.CallOption) ([]LineItem, ResponseMeta, error)
	LineItemMetrics(context.Context, LineItemMetricsRequest, ...socialhub.CallOption) ([]LineItemMetricGroup, ResponseMeta, error)
}

type CreativeWorkflow interface {
	ListCreatives(context.Context, CreativeListRequest, ...socialhub.CallOption) ([]Creative, ResponseMeta, error)
	CreativeMetrics(context.Context, CreativeMetricsRequest, ...socialhub.CallOption) ([]CreativeMetricGroup, ResponseMeta, error)
}

var _ AdvertiserWorkflow = (*Client)(nil)
var _ CampaignWorkflow = (*Client)(nil)
var _ LineItemWorkflow = (*Client)(nil)
var _ CreativeWorkflow = (*Client)(nil)
