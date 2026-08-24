package jinghongads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	ScopeBaseProfile = "https://www.huawei.com/auth/account/base.profile"
	ScopeReport      = "https://ads.cloud.huawei.com/report"
	ScopePromotion   = "https://ads.cloud.huawei.com/promotion"
	ScopeTools       = "https://ads.cloud.huawei.com/tools"
	ScopeAccount     = "https://ads.cloud.huawei.com/account"
	ScopeFinance     = "https://ads.cloud.huawei.com/finance"
)

var requiredOAuthScopes = []string{
	ScopeBaseProfile, ScopeReport, ScopePromotion, ScopeTools, ScopeAccount, ScopeFinance,
}

func RequiredOAuthScopes() []string { return append([]string(nil), requiredOAuthScopes...) }

type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

// ReportValue preserves exact string and JSON-number representations. Mainland
// reports return a mixture of number-encoded counts and string-encoded money.
type ReportValue struct {
	Text string
	Null bool
}

func (value ReportValue) String() string { return value.Text }
func (value ReportValue) IsNull() bool   { return value.Null }

func (value *ReportValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		value.Text, value.Null = "", true
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &value.Text); err != nil {
			return err
		}
		value.Null = false
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil || number.String() == "" {
		return fmt.Errorf("jinghongads: report value must be a string, number, or null")
	}
	value.Text, value.Null = number.String(), false
	return nil
}

func (value ReportValue) MarshalJSON() ([]byte, error) {
	if value.Null {
		return []byte("null"), nil
	}
	return json.Marshal(value.Text)
}

type ReportRow map[string]ReportValue

type AdvertiserAccount struct {
	ID              string `json:"-"`
	Name            string `json:"accountName,omitempty"`
	CorporationName string `json:"corpName,omitempty"`
	Type            string `json:"accountType,omitempty"`
	ServiceType     string `json:"userServiceType,omitempty"`
}

func (account *AdvertiserAccount) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID              json.RawMessage `json:"accountId"`
		Name            string          `json:"accountName"`
		CorporationName string          `json:"corpName"`
		Type            string          `json:"accountType"`
		ServiceType     string          `json:"userServiceType"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	id, err := decodeID(wire.ID)
	if err != nil {
		return err
	}
	*account = AdvertiserAccount{
		ID: id, Name: wire.Name, CorporationName: wire.CorporationName,
		Type: wire.Type, ServiceType: wire.ServiceType,
	}
	return nil
}

type AccountList struct {
	Accounts []AdvertiserAccount
	Total    int
}

type CampaignFilter struct {
	Name             string   `json:"campaign_name,omitempty"`
	IDs              []string `json:"campaign_ids,omitempty"`
	UpdatedBeginTime string   `json:"updated_begin_time,omitempty"`
	UpdatedEndTime   string   `json:"updated_end_time,omitempty"`
	CreatedBeginTime string   `json:"created_begin_time,omitempty"`
	CreatedEndTime   string   `json:"created_end_time,omitempty"`
	ShowStatus       string   `json:"show_status,omitempty"`
	CampaignType     string   `json:"campaign_type,omitempty"`
}

type ListCampaignsRequest struct {
	Page     int
	PageSize int
	Filter   CampaignFilter
}

type Campaign struct {
	ID                string      `json:"-"`
	Name              string      `json:"campaign_name,omitempty"`
	Status            string      `json:"campaign_status,omitempty"`
	ShowStatus        string      `json:"show_status,omitempty"`
	DailyBudgetStatus string      `json:"campaign_daily_budget_status,omitempty"`
	UserBalanceStatus string      `json:"user_balance_status,omitempty"`
	ProductType       string      `json:"product_type,omitempty"`
	TodayDailyBudget  ReportValue `json:"today_daily_budget,omitempty"`
	TomorrowBudget    ReportValue `json:"tomorrow_daily_budget,omitempty"`
	CreatedTime       string      `json:"created_time,omitempty"`
	Type              string      `json:"campaign_type,omitempty"`
	FlowResource      string      `json:"flow_resource,omitempty"`
	StoreID           string      `json:"-"`
}

func (campaign *Campaign) UnmarshalJSON(data []byte) error {
	type Alias Campaign
	var wire struct {
		Alias
		ID      json.RawMessage `json:"campaign_id"`
		StoreID json.RawMessage `json:"store_id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	id, err := decodeID(wire.ID)
	if err != nil {
		return err
	}
	storeID, err := decodeOptionalID(wire.StoreID)
	if err != nil {
		return err
	}
	*campaign = Campaign(wire.Alias)
	campaign.ID, campaign.StoreID = id, storeID
	return nil
}

type CampaignPage struct {
	Campaigns []Campaign
	Page      int
	PageSize  int
	Total     int
	HasMore   bool
}

type TimeGranularity string

const (
	TimeGranularityHourly  TimeGranularity = "STAT_TIME_GRANULARITY_HOURLY"
	TimeGranularityDaily   TimeGranularity = "STAT_TIME_GRANULARITY_DAILY"
	TimeGranularityMonthly TimeGranularity = "STAT_TIME_GRANULARITY_MONTHLY"
	TimeGranularitySummary TimeGranularity = "STAT_TIME_GRANULARITY_SUMMARY"
)

type OrderType string

const (
	OrderAscending  OrderType = "ASC"
	OrderDescending OrderType = "DESC"
)

type CampaignType int

const (
	CampaignTypeDisplay  CampaignType = 1
	CampaignTypeKeyword  CampaignType = 2
	CampaignTypePush     CampaignType = 3
	CampaignTypeShopping CampaignType = 4
)

type Metric string

const (
	MetricCost              Metric = "effective_cost"
	MetricImpressions       Metric = "effective_impression_count"
	MetricClicks            Metric = "effective_click_count"
	MetricCPC               Metric = "effective_per_click"
	MetricDownloads         Metric = "effective_download_count"
	MetricClickRate         Metric = "effective_click_ratio"
	MetricClickDownloadRate Metric = "effective_click_download_ratio"
)

type MetricFilterMode string

const (
	MetricGreaterOrEqual MetricFilterMode = "greater_or_equal"
	MetricLessOrEqual    MetricFilterMode = "less_or_equal"
	MetricBetween        MetricFilterMode = "between"
)

// Decimal is encoded as a JSON number without passing through float64.
type Decimal string

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(string(value)) {
		return nil, fmt.Errorf("jinghongads: invalid decimal")
	}
	return []byte(value), nil
}

// MetricFilter uses a semantic mode because Huawei's wire-level type value is
// different for each report level (1..3, 4..6, 7..9, or 10..12).
type MetricFilter struct {
	Metric    Metric
	Mode      MetricFilterMode
	LowValue  *Decimal
	HighValue *Decimal
}

type DimensionFilter struct {
	Dimension string
	Data      []string
}

type GroupBy string

const (
	GroupByDate         GroupBy = "DATE"
	GroupByHour         GroupBy = "HOUR"
	GroupByAdGroupID    GroupBy = "ADGROUP_ID"
	GroupByCountry      GroupBy = "COUNTRY"
	GroupBySearchWord   GroupBy = "SEARCH_KEY_WORD"
	GroupByDealID       GroupBy = "DEAL_ID"
	GroupByCampaignID   GroupBy = "CAMPAIGN_ID"
	GroupByAdvertiserID GroupBy = "ADVERTISER_ID"
	GroupByCreativeID   GroupBy = "CREATIVE_ID"
	GroupByPositionID   GroupBy = "POSITION_ID"
)

type ReportBase struct {
	StartDate       Date
	EndDate         Date
	TimeGranularity TimeGranularity
	Page            int
	PageSize        int
	OrderField      string
	OrderType       OrderType
	TopN            int
	FlowResource    int
	CampaignType    CampaignType
	MetricFilters   []MetricFilter
	Dimension       *DimensionFilter
	TimeLine        string
	GroupBy         []GroupBy
}

type AdvertiserReportRequest struct{ ReportBase }

type CampaignReportFilter struct {
	CampaignIDs  []string
	CampaignName string
	ProductTypes []string
}

type CampaignReportRequest struct {
	ReportBase
	Filter CampaignReportFilter
}

type AdGroupReportFilter struct {
	CampaignIDs          []string
	CampaignName         string
	AdGroupIDs           []string
	AdGroupName          string
	ProductTypes         []string
	AppIDs               []string
	AppChannelPackageIDs []string
	PlacementName        string
	Pricings             []string
}

type AdGroupReportRequest struct {
	ReportBase
	Filter AdGroupReportFilter
}

type CreativeReportFilter struct {
	CampaignIDs   []string
	CampaignName  string
	AdGroupIDs    []string
	AdGroupName   string
	CreativeIDs   []string
	PlacementName string
	Pricings      []string
}

type CreativeReportRequest struct {
	ReportBase
	Filter CreativeReportFilter
}

type PageInfo struct {
	Page        int
	PageSize    int
	TotalNumber int
	TotalPages  int
}

type ReportPage struct {
	Rows     []ReportRow
	Summary  ReportRow
	PageInfo PageInfo
	HasMore  bool
}

type AccountWorkflow interface {
	ListAccounts(context.Context, ...socialhub.CallOption) (AccountList, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (CampaignPage, error)
}

type ReportWorkflow interface {
	AdvertiserReport(context.Context, AdvertiserReportRequest, ...socialhub.CallOption) (ReportPage, error)
	CampaignReport(context.Context, CampaignReportRequest, ...socialhub.CallOption) (ReportPage, error)
	AdGroupReport(context.Context, AdGroupReportRequest, ...socialhub.CallOption) (ReportPage, error)
	CreativeReport(context.Context, CreativeReportRequest, ...socialhub.CallOption) (ReportPage, error)
}
