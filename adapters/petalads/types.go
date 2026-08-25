package petalads

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

// RequiredOAuthScopes returns the six scopes required by Huawei's documented
// Petal Ads authorization URL.
func RequiredOAuthScopes() []string { return append([]string(nil), requiredOAuthScopes...) }

type Date string

func DateFromTime(value time.Time) Date { return Date(value.UTC().Format("2006-01-02")) }

// ReportValue preserves exact string and JSON-number representations. Petal
// Ads commonly encodes monetary values as strings and counts as numbers.
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
		return fmt.Errorf("petalads: report value must be a string, number, or null")
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
	ID                        string      `json:"-"`
	Name                      string      `json:"campaign_name,omitempty"`
	Status                    string      `json:"campaign_status,omitempty"`
	ShowStatus                string      `json:"show_status,omitempty"`
	DailyBudgetStatus         string      `json:"campaign_daily_budget_status,omitempty"`
	UserBalanceStatus         string      `json:"user_balance_status,omitempty"`
	ProductType               string      `json:"product_type,omitempty"`
	TodayDailyBudget          ReportValue `json:"today_daily_budget,omitempty"`
	TomorrowDailyBudget       ReportValue `json:"tomorrow_daily_budget,omitempty"`
	CreatedTime               string      `json:"created_time,omitempty"`
	Type                      string      `json:"campaign_type,omitempty"`
	MarketingGoal             string      `json:"marketing_goal,omitempty"`
	FlowResource              string      `json:"flow_resource,omitempty"`
	SyncFlowResourceSearchAds string      `json:"sync_flow_resource_searchad,omitempty"`
}

func (campaign *Campaign) UnmarshalJSON(data []byte) error {
	type Alias Campaign
	var wire struct {
		Alias
		ID json.RawMessage `json:"campaign_id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	id, err := decodeID(wire.ID)
	if err != nil {
		return err
	}
	*campaign = Campaign(wire.Alias)
	campaign.ID = id
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
	TimeGranularityDaily   TimeGranularity = "STAT_TIME_GRANULARITY_DAILY"
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
	MetricCost        Metric = "effective_cost"
	MetricImpressions Metric = "effective_impression_count"
	MetricClicks      Metric = "effective_click_count"
	MetricCPC         Metric = "effective_per_click"
	MetricDownloads   Metric = "effective_download_count"
)

type MetricFilterType int

const (
	MetricBetween        MetricFilterType = 0
	MetricGreaterOrEqual MetricFilterType = 1
	MetricLessOrEqual    MetricFilterType = 2
)

// Decimal is encoded as a JSON number without passing through float64.
type Decimal string

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(string(value)) {
		return nil, fmt.Errorf("petalads: invalid decimal")
	}
	return []byte(value), nil
}

type MetricFilter struct {
	Metric    Metric           `json:"index_screen"`
	Type      MetricFilterType `json:"type"`
	LowValue  *Decimal         `json:"low_value,omitempty"`
	HighValue *Decimal         `json:"up_value,omitempty"`
}

type DimensionFilter struct {
	Dimension string   `json:"dimension"`
	Data      []string `json:"data"`
}

// ReportBase contains fields shared by the five synchronous reporting
// endpoints. Page and PageSize are not accepted by the Country report.
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
	GroupBy         []string
	TargetCountries []string
}

type AdvertiserReportRequest struct {
	ReportBase
}

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

type CountryFilterType string

const (
	CountryFilterCampaign CountryFilterType = "CAMPAIGN"
	CountryFilterAdGroup  CountryFilterType = "ADGROUP"
	CountryFilterCreative CountryFilterType = "CREATIVE"
)

type CountryReportFilter struct {
	Type        CountryFilterType
	CampaignIDs []string
	AdGroupIDs  []string
	CreativeIDs []string
}

type CountryReportRequest struct {
	ReportBase
	Filter CountryReportFilter
}

type PageInfo struct {
	Page        int `json:"page"`
	PageSize    int `json:"page_size"`
	TotalNumber int `json:"total_number"`
	TotalPages  int `json:"total_page"`
}

type ReportPage struct {
	Rows     []ReportRow
	Summary  ReportRow
	PageInfo PageInfo
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
	CountryReport(context.Context, CountryReportRequest, ...socialhub.CallOption) (ReportPage, error)
}
