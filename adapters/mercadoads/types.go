package mercadoads

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
		return fmt.Errorf("mercadoads: invalid exact value")
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
		return fmt.Errorf("mercadoads: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

type Metric string

const (
	MetricClicks                      Metric = "clicks"
	MetricPrints                      Metric = "prints"
	MetricCTR                         Metric = "ctr"
	MetricCost                        Metric = "cost"
	MetricCostUSD                     Metric = "cost_usd"
	MetricCPC                         Metric = "cpc"
	MetricACOS                        Metric = "acos"
	MetricOrganicUnitsQuantity        Metric = "organic_units_quantity"
	MetricOrganicUnitsAmount          Metric = "organic_units_amount"
	MetricOrganicItemsQuantity        Metric = "organic_items_quantity"
	MetricDirectItemsQuantity         Metric = "direct_items_quantity"
	MetricIndirectItemsQuantity       Metric = "indirect_items_quantity"
	MetricAdvertisingItemsQuantity    Metric = "advertising_items_quantity"
	MetricCVR                         Metric = "cvr"
	MetricROAS                        Metric = "roas"
	MetricSOV                         Metric = "sov"
	MetricDirectUnitsQuantity         Metric = "direct_units_quantity"
	MetricIndirectUnitsQuantity       Metric = "indirect_units_quantity"
	MetricUnitsQuantity               Metric = "units_quantity"
	MetricDirectAmount                Metric = "direct_amount"
	MetricIndirectAmount              Metric = "indirect_amount"
	MetricTotalAmount                 Metric = "total_amount"
	MetricImpressionShare             Metric = "impression_share"
	MetricTopImpressionShare          Metric = "top_impression_share"
	MetricLostImpressionShareByBudget Metric = "lost_impression_share_by_budget"
	MetricLostImpressionShareByAdRank Metric = "lost_impression_share_by_ad_rank"
	MetricACOSBenchmark               Metric = "acos_benchmark"
)

// Metrics is the union of documented Product Ads campaign and item metrics.
// ExactValue distinguishes omitted, null, zero, integer, and decimal fields.
type Metrics struct {
	Clicks                      ExactValue `json:"clicks"`
	Prints                      ExactValue `json:"prints"`
	CTR                         ExactValue `json:"ctr"`
	Cost                        ExactValue `json:"cost"`
	CostUSD                     ExactValue `json:"cost_usd"`
	CPC                         ExactValue `json:"cpc"`
	ACOS                        ExactValue `json:"acos"`
	OrganicUnitsQuantity        ExactValue `json:"organic_units_quantity"`
	OrganicUnitsAmount          ExactValue `json:"organic_units_amount"`
	OrganicItemsQuantity        ExactValue `json:"organic_items_quantity"`
	DirectItemsQuantity         ExactValue `json:"direct_items_quantity"`
	IndirectItemsQuantity       ExactValue `json:"indirect_items_quantity"`
	AdvertisingItemsQuantity    ExactValue `json:"advertising_items_quantity"`
	CVR                         ExactValue `json:"cvr"`
	ROAS                        ExactValue `json:"roas"`
	SOV                         ExactValue `json:"sov"`
	DirectUnitsQuantity         ExactValue `json:"direct_units_quantity"`
	IndirectUnitsQuantity       ExactValue `json:"indirect_units_quantity"`
	UnitsQuantity               ExactValue `json:"units_quantity"`
	DirectAmount                ExactValue `json:"direct_amount"`
	IndirectAmount              ExactValue `json:"indirect_amount"`
	TotalAmount                 ExactValue `json:"total_amount"`
	ImpressionShare             ExactValue `json:"impression_share"`
	TopImpressionShare          ExactValue `json:"top_impression_share"`
	LostImpressionShareByBudget ExactValue `json:"lost_impression_share_by_budget"`
	LostImpressionShareByAdRank ExactValue `json:"lost_impression_share_by_ad_rank"`
	ACOSBenchmark               ExactValue `json:"acos_benchmark"`
}

type DailyMetrics struct {
	Date Date `json:"date"`
	Metrics
}

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
	Total      int64      `json:"total"`
	Offset     int64      `json:"offset"`
	Limit      int64      `json:"limit"`
	LastItemID ExactValue `json:"last_item_id"`
}

type CampaignStatus string

const (
	CampaignStatusActive CampaignStatus = "active"
	CampaignStatusPaused CampaignStatus = "paused"
)

type Campaign struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Status      CampaignStatus `json:"status"`
	Budget      ExactValue     `json:"budget"`
	CurrencyID  string         `json:"currency_id"`
	LastUpdated string         `json:"last_updated"`
	DateCreated string         `json:"date_created"`
	ACOSTarget  ExactValue     `json:"acos_target"`
	Strategy    string         `json:"strategy"`
	Channel     string         `json:"channel"`
	Metrics     *Metrics       `json:"metrics,omitempty"`
	Meta        ResponseMeta   `json:"-"`
}

type CampaignPage struct {
	Paging         Paging       `json:"paging"`
	Results        []Campaign   `json:"results"`
	MetricsSummary *Metrics     `json:"metrics_summary,omitempty"`
	Meta           ResponseMeta `json:"-"`
}

type DailyMetricsPage struct {
	Paging  Paging         `json:"paging"`
	Results []DailyMetrics `json:"results"`
	Meta    ResponseMeta   `json:"-"`
}

type ItemStatus string

const (
	ItemStatusActive    ItemStatus = "active"
	ItemStatusPaused    ItemStatus = "paused"
	ItemStatusHold      ItemStatus = "hold"
	ItemStatusIdle      ItemStatus = "idle"
	ItemStatusDelegated ItemStatus = "delegated"
	ItemStatusRevoked   ItemStatus = "revoked"
)

type Item struct {
	ItemID          string       `json:"item_id"`
	CampaignID      *int64       `json:"campaign_id"`
	Price           ExactValue   `json:"price"`
	Title           string       `json:"title"`
	Status          ItemStatus   `json:"status"`
	HasDiscount     bool         `json:"has_discount"`
	CatalogListing  bool         `json:"catalog_listing"`
	LogisticType    string       `json:"logistic_type"`
	ListingTypeID   string       `json:"listing_type_id"`
	DomainID        string       `json:"domain_id"`
	DateCreated     string       `json:"date_created"`
	BuyBoxWinner    bool         `json:"buy_box_winner"`
	Tags            []string     `json:"tags"`
	Channel         string       `json:"channel"`
	OfficialStoreID *int64       `json:"official_store_id"`
	BrandValueID    string       `json:"brand_value_id"`
	BrandValueName  string       `json:"brand_value_name"`
	Condition       string       `json:"condition"`
	CurrentLevel    string       `json:"current_level"`
	DeferredStock   bool         `json:"deferred_stock"`
	PictureID       string       `json:"picture_id"`
	Thumbnail       string       `json:"thumbnail"`
	Permalink       string       `json:"permalink"`
	Recommended     bool         `json:"recommended"`
	Metrics         *Metrics     `json:"metrics,omitempty"`
	MetricsSummary  *Metrics     `json:"metrics_summary,omitempty"`
	Meta            ResponseMeta `json:"-"`
}

type ItemPage struct {
	Paging         Paging       `json:"paging"`
	Results        []Item       `json:"results"`
	MetricsSummary *Metrics     `json:"metrics_summary,omitempty"`
	Meta           ResponseMeta `json:"-"`
}

type MetricQuery struct {
	DateFrom Date
	DateTo   Date
	Metrics  []Metric
}

type CampaignListRequest struct {
	Limit          int64
	Offset         int64
	DateFrom       Date
	DateTo         Date
	Metrics        []Metric
	MetricsSummary bool
	CampaignIDs    []int64
	Statuses       []CampaignStatus
	Channel        string
}

type ItemFilters struct {
	ItemIDs        []string
	Statuses       []ItemStatus
	Channel        string
	BuyBoxWinner   *bool
	Condition      string
	CurrentLevel   string
	DeferredStock  *bool
	Domains        []string
	LogisticTypes  []string
	ListingTypes   []string
	OfficialStores []int64
	Recommended    *bool
	CampaignID     int64
	Campaigns      []int64
	BrandValueID   string
	BrandValueName string
}

type ItemListRequest struct {
	Limit          int64
	Offset         int64
	DateFrom       Date
	DateTo         Date
	Metrics        []Metric
	MetricsSummary bool
	Filters        ItemFilters
}

type AdvertiserWorkflow interface {
	ListAdvertisers(context.Context, ...socialhub.CallOption) (AdvertiserList, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, CampaignListRequest, ...socialhub.CallOption) (CampaignPage, error)
	ListCampaignDailyMetrics(context.Context, CampaignListRequest, ...socialhub.CallOption) (DailyMetricsPage, error)
	GetCampaign(context.Context, int64, MetricQuery, ...socialhub.CallOption) (*Campaign, error)
	GetCampaignDailyMetrics(context.Context, int64, MetricQuery, ...socialhub.CallOption) ([]DailyMetrics, ResponseMeta, error)
}

type ItemWorkflow interface {
	ListItems(context.Context, ItemListRequest, ...socialhub.CallOption) (ItemPage, error)
	ListItemDailyMetrics(context.Context, ItemListRequest, ...socialhub.CallOption) (DailyMetricsPage, error)
	GetItem(context.Context, string, ...socialhub.CallOption) (*Item, error)
	GetItemMetrics(context.Context, string, MetricQuery, ...socialhub.CallOption) (*Item, error)
	GetItemDailyMetrics(context.Context, string, MetricQuery, ...socialhub.CallOption) ([]DailyMetrics, ResponseMeta, error)
}

var _ AdvertiserWorkflow = (*Client)(nil)
var _ CampaignWorkflow = (*Client)(nil)
var _ ItemWorkflow = (*Client)(nil)
