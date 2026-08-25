package mercadobrandads

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
		return fmt.Errorf("mercadobrandads: invalid exact value")
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

// IsNumber reports whether the preserved scalar is a JSON number.
func (value ExactValue) IsNumber() bool {
	trimmed := bytes.TrimSpace(value.raw)
	return len(trimmed) > 0 && (trimmed[0] == '-' || trimmed[0] >= '0' && trimmed[0] <= '9')
}

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
		return fmt.Errorf("mercadobrandads: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

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

type Paging struct {
	Total  int64 `json:"total"`
	Offset int64 `json:"offset"`
	Limit  int64 `json:"limit"`
}

type CampaignType string

const (
	CampaignTypeAutomatic CampaignType = "automatic"
	CampaignTypeCustom    CampaignType = "custom"
)

type Budget struct {
	Amount   ExactValue `json:"amount"`
	Currency string     `json:"currency"`
}

type Item struct {
	CampaignID int64  `json:"campaign_id"`
	Status     string `json:"status"`
	ItemID     string `json:"item_id"`
}

type Keyword struct {
	CampaignID int64      `json:"campaign_id"`
	KeywordID  int64      `json:"keyword_id"`
	Type       string     `json:"type"`
	Term       string     `json:"term"`
	MatchType  string     `json:"match_type"`
	IsNegative bool       `json:"is_negative"`
	CPC        ExactValue `json:"cpc"`
}

type Campaign struct {
	CampaignID      int64        `json:"campaign_id"`
	Name            string       `json:"name"`
	StartDate       string       `json:"start_date"`
	EndDate         string       `json:"end_date"`
	AdvertiserID    int64        `json:"advertiser_id"`
	CampaignType    CampaignType `json:"campaign_type"`
	Status          string       `json:"status"`
	SiteID          string       `json:"site_id"`
	OfficialStoreID int64        `json:"official_store_id"`
	DestinationID   int64        `json:"destination_id"`
	Headline        string       `json:"headline"`
	Budget          Budget       `json:"budget"`
	CPC             ExactValue   `json:"cpc"`
	Items           []Item       `json:"items"`
	Keywords        []Keyword    `json:"keywords"`
	Meta            ResponseMeta `json:"-"`
}

type CampaignPage struct {
	Paging    Paging       `json:"paging"`
	Campaigns []Campaign   `json:"campaigns"`
	Meta      ResponseMeta `json:"-"`
}

// AttributionMetric is used by both event_time and touch_point attribution.
type AttributionMetric struct {
	UnitsQuantity            ExactValue `json:"units_quantity"`
	UnitsAmount              ExactValue `json:"units_amount"`
	ItemsQuantity            ExactValue `json:"items_quantity"`
	PPVConversions           ExactValue `json:"ppv_conversions"`
	BookmarkConversions      ExactValue `json:"bookmark_conversions"`
	CartConversions          ExactValue `json:"cart_conversions"`
	CheckoutConversions      ExactValue `json:"checkout_conversions"`
	LeadsQuestionConversions ExactValue `json:"leads_question_conversions"`
	LeadsIMConversions       ExactValue `json:"leads_im_conversions"`
	EshopConversions         ExactValue `json:"eshop_conversions"`
}

type CompetitiveMetric struct {
	LostImpressionShareByBudget ExactValue `json:"lost_impression_share_by_budget"`
	LostImpressionShareByAdRank ExactValue `json:"lost_impression_share_by_ad_rank"`
	ImpressionShare             ExactValue `json:"impression_share"`
	CompetitiveCPC              ExactValue `json:"competitive_cpc"`
}

// Metric contains one Brand Ads metric row or summary. All provider numeric
// values remain exact JSON scalars.
type Metric struct {
	Date           Date               `json:"date"`
	Keyword        string             `json:"keyword"`
	SiteID         string             `json:"site_id"`
	Currency       string             `json:"currency"`
	Prints         ExactValue         `json:"prints"`
	Clicks         ExactValue         `json:"clicks"`
	CTR            ExactValue         `json:"ctr"`
	CVR            ExactValue         `json:"cvr"`
	ConsumedBudget ExactValue         `json:"consumed_budget"`
	CPC            ExactValue         `json:"cpc"`
	ACOS           ExactValue         `json:"acos"`
	EventTime      AttributionMetric  `json:"event_time"`
	TouchPoint     AttributionMetric  `json:"touch_point"`
	Competitive    *CompetitiveMetric `json:"competitive,omitempty"`
}

type CampaignMetricPage struct {
	Paging  Paging       `json:"paging"`
	Metrics []Metric     `json:"metrics"`
	Summary *Metric      `json:"summary"`
	Meta    ResponseMeta `json:"-"`
}

type KeywordDailyMetric struct {
	Date     Date     `json:"date"`
	Keywords []Metric `json:"keywords"`
}

type KeywordMetricPage struct {
	Paging  Paging               `json:"paging"`
	Metrics []KeywordDailyMetric `json:"metrics"`
	Summary []Metric             `json:"summary"`
	Meta    ResponseMeta         `json:"-"`
}

type AggregationType string

const (
	AggregationDaily AggregationType = "daily"
	AggregationTotal AggregationType = "total"
)

type MetricField string

const (
	MetricFieldPrints         MetricField = "prints"
	MetricFieldClicks         MetricField = "clicks"
	MetricFieldCTR            MetricField = "ctr"
	MetricFieldCVR            MetricField = "cvr"
	MetricFieldConsumedBudget MetricField = "consumed_budget"
	MetricFieldCPC            MetricField = "cpc"
	MetricFieldACOS           MetricField = "acos"
	MetricFieldEventTime      MetricField = "event_time"
	MetricFieldTouchPoint     MetricField = "touch_point"
	MetricFieldCompetitive    MetricField = "competitive"
)

type MetricQuery struct {
	DateFrom        Date
	DateTo          Date
	Limit           int64
	Offset          int64
	AggregationType AggregationType
	Fields          []MetricField
}

type CampaignFilterStatus string

const (
	CampaignFilterActive CampaignFilterStatus = "active"
	CampaignFilterPaused CampaignFilterStatus = "paused"
)

type AdvertiserMetricRequest struct {
	MetricQuery
	Status        CampaignFilterStatus
	DestinationID int64
}

type CampaignMetricRequest struct {
	MetricQuery
	CampaignID int64
}

type KeywordMetricRequest struct {
	MetricQuery
	CampaignID int64
}

type AdvertiserWorkflow interface {
	ListAdvertisers(context.Context, ...socialhub.CallOption) (AdvertiserList, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ...socialhub.CallOption) (CampaignPage, error)
	GetCampaign(context.Context, int64, ...socialhub.CallOption) (*Campaign, error)
	ListAdvertiserCampaignMetrics(context.Context, AdvertiserMetricRequest, ...socialhub.CallOption) (CampaignMetricPage, error)
	GetCampaignMetrics(context.Context, CampaignMetricRequest, ...socialhub.CallOption) (CampaignMetricPage, error)
}

type ItemWorkflow interface {
	ListItems(context.Context, int64, ...socialhub.CallOption) ([]Item, ResponseMeta, error)
}

type KeywordWorkflow interface {
	ListKeywords(context.Context, int64, ...socialhub.CallOption) ([]Keyword, ResponseMeta, error)
	GetKeywordMetrics(context.Context, KeywordMetricRequest, ...socialhub.CallOption) (KeywordMetricPage, error)
}

var _ AdvertiserWorkflow = (*Client)(nil)
var _ CampaignWorkflow = (*Client)(nil)
var _ ItemWorkflow = (*Client)(nil)
var _ KeywordWorkflow = (*Client)(nil)
