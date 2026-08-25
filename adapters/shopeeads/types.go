package shopeeads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const maxExactValueBytes = 256

// ExactValue preserves provider numeric values without float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) ||
		trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9') {
		return fmt.Errorf("shopeeads: invalid exact value")
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
func (value ExactValue) IsNumber() bool {
	trimmed := bytes.TrimSpace(value.raw)
	return len(trimmed) > 0 && json.Valid(trimmed) &&
		(trimmed[0] == '-' || trimmed[0] >= '0' && trimmed[0] <= '9')
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
		return fmt.Errorf("shopeeads: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID string
	Warning   string
}

type Balance struct {
	DataTimestamp int64        `json:"data_timestamp"`
	TotalBalance  ExactValue   `json:"total_balance"`
	Meta          ResponseMeta `json:"-"`
}

type ShopToggleInfo struct {
	DataTimestamp int64        `json:"data_timestamp"`
	AutoTopUp     bool         `json:"auto_top_up"`
	CampaignSurge bool         `json:"campaign_surge"`
	Meta          ResponseMeta `json:"-"`
}

type SuggestedKeyword struct {
	Keyword      string     `json:"keyword"`
	QualityScore int64      `json:"quality_score"`
	SearchVolume int64      `json:"search_volume"`
	SuggestedBid ExactValue `json:"suggested_bid"`
}

type KeywordRecommendations struct {
	ItemID            int64              `json:"item_id"`
	InputKeyword      string             `json:"input_keyword"`
	SuggestedKeywords []SuggestedKeyword `json:"suggested_keywords"`
	Meta              ResponseMeta       `json:"-"`
}

type RecommendedItem struct {
	ItemID            int64    `json:"item_id"`
	ItemStatusList    []string `json:"item_status_list"`
	SKUTagList        []string `json:"sku_tag_list"`
	OngoingAdTypeList []string `json:"ongoing_ad_type_list"`
}

type RecommendedItems struct {
	Items []RecommendedItem
	Meta  ResponseMeta
}

type CPCPerformance struct {
	Hour              *int       `json:"hour,omitempty"`
	Date              string     `json:"date"`
	Impression        ExactValue `json:"impression"`
	Clicks            ExactValue `json:"clicks"`
	CTR               ExactValue `json:"ctr"`
	DirectOrder       ExactValue `json:"direct_order"`
	BroadOrder        ExactValue `json:"broad_order"`
	DirectConversions ExactValue `json:"direct_conversions"`
	BroadConversions  ExactValue `json:"broad_conversions"`
	DirectItemSold    ExactValue `json:"direct_item_sold"`
	BroadItemSold     ExactValue `json:"broad_item_sold"`
	DirectGMV         ExactValue `json:"direct_gmv"`
	BroadGMV          ExactValue `json:"broad_gmv"`
	Expense           ExactValue `json:"expense"`
	CostPerConversion ExactValue `json:"cost_per_conversion"`
	DirectROAS        ExactValue `json:"direct_roas"`
	BroadROAS         ExactValue `json:"broad_roas"`
}

type CPCPerformanceResult struct {
	Rows []CPCPerformance
	Meta ResponseMeta
}

type ProductCampaignMetric struct {
	Hour              *int       `json:"hour,omitempty"`
	Date              string     `json:"date"`
	Impression        ExactValue `json:"impression"`
	Clicks            ExactValue `json:"clicks"`
	CTR               ExactValue `json:"ctr"`
	Expense           ExactValue `json:"expense"`
	BroadGMV          ExactValue `json:"broad_gmv"`
	BroadOrder        ExactValue `json:"broad_order"`
	BroadOrderAmount  ExactValue `json:"broad_order_amount"`
	BroadROI          ExactValue `json:"broad_roi"`
	BroadCIR          ExactValue `json:"broad_cir"`
	CR                ExactValue `json:"cr"`
	CPC               ExactValue `json:"cpc"`
	DirectOrder       ExactValue `json:"direct_order"`
	DirectOrderAmount ExactValue `json:"direct_order_amount"`
	DirectGMV         ExactValue `json:"direct_gmv"`
	DirectROI         ExactValue `json:"direct_roi"`
	DirectCIR         ExactValue `json:"direct_cir"`
	DirectCR          ExactValue `json:"direct_cr"`
	CPDC              ExactValue `json:"cpdc"`
}

type ProductCampaignPerformance struct {
	CampaignID        int64                   `json:"campaign_id"`
	AdType            string                  `json:"ad_type"`
	CampaignPlacement string                  `json:"campaign_placement"`
	AdName            string                  `json:"ad_name"`
	Metrics           []ProductCampaignMetric `json:"metrics_list"`
}

type ProductCampaignPerformanceShop struct {
	ShopID    int64                        `json:"shop_id"`
	Region    string                       `json:"region"`
	Campaigns []ProductCampaignPerformance `json:"campaign_list"`
}

type ProductCampaignPerformanceResult struct {
	Shops []ProductCampaignPerformanceShop
	Meta  ResponseMeta
}

type CampaignPerformanceRequest struct {
	CampaignIDs     []int64
	StartDate       string
	EndDate         string
	PerformanceDate string
}

type CampaignIDListRequest struct {
	AdType string
	Offset int64
	Limit  int64
}

type CampaignID struct {
	AdType     string `json:"ad_type"`
	CampaignID int64  `json:"campaign_id"`
}

type CampaignIDList struct {
	ShopID      int64        `json:"shop_id"`
	Region      string       `json:"region"`
	HasNextPage bool         `json:"has_next_page"`
	Campaigns   []CampaignID `json:"campaign_list"`
	Meta        ResponseMeta `json:"-"`
}

type CampaignInfoType int

const (
	CampaignInfoCommon      CampaignInfoType = 1
	CampaignInfoManualBid   CampaignInfoType = 2
	CampaignInfoAutoBid     CampaignInfoType = 3
	CampaignInfoAutoProduct CampaignInfoType = 4
)

type CampaignSettingsRequest struct {
	CampaignIDs []int64
	InfoTypes   []CampaignInfoType
}

type CampaignDuration struct {
	StartTime int64 `json:"start_time"`
	EndTime   int64 `json:"end_time"`
}

type CampaignCommonInfo struct {
	AdType            string           `json:"ad_type"`
	AdName            string           `json:"ad_name"`
	CampaignStatus    string           `json:"campaign_status"`
	BiddingMethod     string           `json:"bidding_method"`
	CampaignPlacement string           `json:"campaign_placement"`
	CampaignBudget    ExactValue       `json:"campaign_budget"`
	CampaignDuration  CampaignDuration `json:"campaign_duration"`
	ItemIDs           []int64          `json:"item_id_list"`
}

type SelectedKeyword struct {
	Keyword          string     `json:"keyword"`
	Status           string     `json:"status"`
	MatchType        string     `json:"match_type"`
	BidPricePerClick ExactValue `json:"bid_price_per_click"`
}

type DiscoveryAdsLocation struct {
	Location string     `json:"location"`
	Status   string     `json:"status"`
	BidPrice ExactValue `json:"bid_price"`
}

type ManualBiddingInfo struct {
	EnhancedCPC           bool                   `json:"enhanced_cpc"`
	SelectedKeywords      []SelectedKeyword      `json:"selected_keywords"`
	DiscoveryAdsLocations []DiscoveryAdsLocation `json:"discovery_ads_locations"`
}

type AutoBiddingInfo struct {
	ROASTarget ExactValue `json:"roas_target"`
}

type AutoProductAdsInfo struct {
	ProductName string `json:"product_name"`
	Status      string `json:"status"`
	ItemID      int64  `json:"item_id"`
}

type ProductCampaignSettings struct {
	CampaignID        int64                `json:"campaign_id"`
	CommonInfo        *CampaignCommonInfo  `json:"common_info,omitempty"`
	ManualBiddingInfo *ManualBiddingInfo   `json:"manual_bidding_info,omitempty"`
	AutoBiddingInfo   *AutoBiddingInfo     `json:"auto_bidding_info,omitempty"`
	AutoProductAds    []AutoProductAdsInfo `json:"auto_product_ads_info"`
}

type CampaignSettings struct {
	ShopID    int64                     `json:"shop_id"`
	Region    string                    `json:"region"`
	Campaigns []ProductCampaignSettings `json:"campaign_list"`
	Meta      ResponseMeta              `json:"-"`
}

type BalanceWorkflow interface {
	GetTotalBalance(context.Context, ...socialhub.CallOption) (*Balance, error)
	GetShopToggleInfo(context.Context, ...socialhub.CallOption) (*ShopToggleInfo, error)
}

type RecommendationWorkflow interface {
	GetRecommendedKeywords(context.Context, int64, string, ...socialhub.CallOption) (*KeywordRecommendations, error)
	GetRecommendedItems(context.Context, ...socialhub.CallOption) (RecommendedItems, error)
}

type CampaignWorkflow interface {
	ListProductCampaignIDs(context.Context, CampaignIDListRequest, ...socialhub.CallOption) (*CampaignIDList, error)
	GetProductCampaignSettings(context.Context, CampaignSettingsRequest, ...socialhub.CallOption) (*CampaignSettings, error)
}

type PerformanceReportWorkflow interface {
	GetAllCPCHourlyPerformance(context.Context, string, ...socialhub.CallOption) (CPCPerformanceResult, error)
	GetAllCPCDailyPerformance(context.Context, string, string, ...socialhub.CallOption) (CPCPerformanceResult, error)
	GetProductCampaignDailyPerformance(context.Context, CampaignPerformanceRequest, ...socialhub.CallOption) (ProductCampaignPerformanceResult, error)
	GetProductCampaignHourlyPerformance(context.Context, CampaignPerformanceRequest, ...socialhub.CallOption) (ProductCampaignPerformanceResult, error)
}
