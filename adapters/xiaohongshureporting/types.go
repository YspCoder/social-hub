package xiaohongshureporting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

type Date string

var chinaTimeZone = time.FixedZone("Asia/Shanghai", 8*60*60)

func DateFromTime(value time.Time) Date { return Date(value.In(chinaTimeZone).Format("2006-01-02")) }

type TimeUnit string
type SortOrder string
type DataCaliber int

const (
	TimeUnitDay     TimeUnit = "DAY"
	TimeUnitHour    TimeUnit = "HOUR"
	TimeUnitSummary TimeUnit = "SUMMARY"

	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"

	DataCaliberBillingTime    DataCaliber = 0
	DataCaliberConversionTime DataCaliber = 1
)

// FilterClause is a Spotlight report field predicate. Supported columns and
// operators are endpoint-specific and are validated by Spotlight.
type FilterClause struct {
	Column   string   `json:"column"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// OfflineReportRequest contains the filters shared by Spotlight's account,
// campaign, unit, creative, keyword, and note offline report endpoints.
type OfflineReportRequest struct {
	StartDate       Date
	EndDate         Date
	TimeUnit        TimeUnit
	MarketingTarget []int
	BiddingStrategy []int
	OptimizeTarget  []int
	Placement       []int
	PromotionTarget []int
	Programmatic    []int
	BuildType       []int
	DeliveryMode    []int
	SplitColumns    []string
	SortColumn      string
	Sort            SortOrder
	PageNum         int
	PageSize        int
	DataCaliber     DataCaliber
	Filters         []FilterClause
}

// OfflineSimpleReportRequest is used by the SPU report, whose filter surface
// is intentionally narrower than delivery reports.
type OfflineSimpleReportRequest struct {
	StartDate  Date
	EndDate    Date
	TimeUnit   TimeUnit
	SortColumn string
	Sort       SortOrder
	PageNum    int
	PageSize   int
}

// OfflineSearchWordRequest models the search-word endpoint's supported filters.
type OfflineSearchWordRequest struct {
	StartDate       Date
	EndDate         Date
	TimeUnit        TimeUnit
	MarketingTarget []int
	BiddingStrategy []int
	OptimizeTarget  []int
	Placement       []int
	PromotionTarget []int
	Programmatic    []int
	BuildType       []int
	SortColumn      string
	Sort            SortOrder
	PageNum         int
	PageSize        int
	DataCaliber     DataCaliber
}

type RealtimeAccountRequest struct {
	StartDate      Date
	EndDate        Date
	NeedHourlyData bool
	DataCaliber    DataCaliber
}

type RealtimeCampaignRequest struct {
	StartDate               Date
	EndDate                 Date
	SortColumn              string
	Sort                    SortOrder
	PageNum                 int
	PageSize                int
	MarketingTargetList     []int
	CampaignFilterState     int
	CampaignCreateBeginTime string
	CampaignCreateEndTime   string
	PlacementList           []int
	LimitDayBudgetList      []int
	OptimizeTargetList      []int
	BuildTypeList           []int
	BiddingStrategyList     []int
	ConstraintTypeList      []int
	PromotionTargetList     []int
	CombineAuditStatus      int
	MigrationStatusList     []int
	Name                    string
	ID                      int64
	DataCaliber             DataCaliber
	NeedHourlyData          bool
}

type RealtimeUnitRequest struct {
	StartDate           Date
	EndDate             Date
	PageNum             int
	PageSize            int
	SortColumn          string
	Sort                SortOrder
	MarketingTargetList []int
	UnitFilterState     int
	UnitCreateBeginTime string
	UnitCreateEndTime   string
	PlacementList       []int
	BiddingStrategyList []int
	PromotionTargetList []int
	CombineAuditStatus  int
	Name                string
	ID                  int64
	DataCaliber         DataCaliber
	NeedHourlyData      bool
}

type RealtimeCreativeRequest struct {
	StartDate                 Date
	EndDate                   Date
	PageNum                   int
	PageSize                  int
	SortColumn                string
	Sort                      SortOrder
	PlacementList             []int
	CreativityFilterState     int
	CreativityCreateBeginTime string
	CreativityCreateEndTime   string
	ConversionType            int
	ProgrammaticList          []int
	CreativityAuditState      int
	Name                      string
	ID                        int64
	DataCaliber               DataCaliber
	NeedHourlyData            bool
}

type RealtimeKeywordRequest struct {
	StartDate          Date
	EndDate            Date
	PageNum            int
	PageSize           int
	SortColumn         string
	Sort               SortOrder
	KeywordFilterState int
	UseBidStrategy     int
	KeywordName        string
	CampaignName       string
	UnitName           string
	DataCaliber        DataCaliber
	NeedHourlyData     bool
}

type RealtimeTargetRequest struct {
	StartDate           Date
	EndDate             Date
	PageNum             int
	PageSize            int
	SortColumn          string
	Sort                SortOrder
	Name                string
	MarketingTargetList []int
	NeedHourlyData      bool
}

// ReportValue preserves exact JSON, including numbers and nested realtime DTOs.
type ReportValue struct {
	raw json.RawMessage
}

func (value *ReportValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || len(data) > maxReportValueBytes || !json.Valid(data) {
		return fmt.Errorf("xiaohongshureporting: invalid report value")
	}
	value.raw = append(value.raw[:0], data...)
	return nil
}

func (value ReportValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value ReportValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value ReportValue) IsNull() bool {
	return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null"))
}

func (value ReportValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value ReportValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("xiaohongshureporting: decode target and report value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ReportRow map[string]ReportValue

type PageInfo struct {
	PageIndex  int64
	TotalCount int64
}

// ReportPage normalizes the different offline and realtime response shapes.
// Account contains the account-level aggregate; Rows contains endpoint rows;
// Aggregation and Hourly preserve their platform-specific dynamic fields.
type ReportPage struct {
	Account     ReportRow
	Rows        []ReportRow
	Aggregation ReportRow
	Hourly      []ReportRow
	Page        PageInfo
	RequestID   string
}

type AccountBalance struct {
	TotalBalance            ReportValue `json:"total_balance"`
	CashBalance             ReportValue `json:"cash_balance"`
	ReturnBalance           ReportValue `json:"return_balance"`
	CreditBalance           ReportValue `json:"credit_balance"`
	FreezeBalance           ReportValue `json:"freeze_balance"`
	AvailableBalance        ReportValue `json:"available_balance"`
	TodaySpend              ReportValue `json:"today_spend"`
	CompensateReturnBalance ReportValue `json:"compensate_return_balance"`
	AccountBudget           ReportValue `json:"account_budget"`
	LimitDayBudget          ReportValue `json:"limit_day_budget"`
	RequestID               string      `json:"-"`
}

type AccountWorkflow interface {
	Balance(context.Context, ...socialhub.CallOption) (AccountBalance, error)
}

type ReportWorkflow interface {
	OfflineAccount(context.Context, OfflineReportRequest, ...socialhub.CallOption) (ReportPage, error)
	OfflineCampaign(context.Context, OfflineReportRequest, ...socialhub.CallOption) (ReportPage, error)
	OfflineUnit(context.Context, OfflineReportRequest, ...socialhub.CallOption) (ReportPage, error)
	OfflineCreative(context.Context, OfflineReportRequest, ...socialhub.CallOption) (ReportPage, error)
	OfflineKeyword(context.Context, OfflineReportRequest, ...socialhub.CallOption) (ReportPage, error)
	OfflineNote(context.Context, OfflineReportRequest, ...socialhub.CallOption) (ReportPage, error)
	OfflineSPU(context.Context, OfflineSimpleReportRequest, ...socialhub.CallOption) (ReportPage, error)
	OfflineSearchWord(context.Context, OfflineSearchWordRequest, ...socialhub.CallOption) (ReportPage, error)
	RealtimeAccount(context.Context, RealtimeAccountRequest, ...socialhub.CallOption) (ReportPage, error)
	RealtimeCampaign(context.Context, RealtimeCampaignRequest, ...socialhub.CallOption) (ReportPage, error)
	RealtimeUnit(context.Context, RealtimeUnitRequest, ...socialhub.CallOption) (ReportPage, error)
	RealtimeCreative(context.Context, RealtimeCreativeRequest, ...socialhub.CallOption) (ReportPage, error)
	RealtimeKeyword(context.Context, RealtimeKeywordRequest, ...socialhub.CallOption) (ReportPage, error)
	RealtimeTarget(context.Context, RealtimeTargetRequest, ...socialhub.CallOption) (ReportPage, error)
}

var _ AccountWorkflow = (*Client)(nil)
var _ ReportWorkflow = (*Client)(nil)
