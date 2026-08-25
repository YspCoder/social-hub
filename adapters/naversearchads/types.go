package naversearchads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"social-hub/pkg/socialhub"
)

const (
	MaximumKeywordCreateBatch = 100
	MaximumKeywordUpdateBatch = 200
	DefaultMaxReportBytes     = int64(256 << 20)
)

type CampaignType string

const (
	CampaignWebSite      CampaignType = "WEB_SITE"
	CampaignShopping     CampaignType = "SHOPPING"
	CampaignBrandSearch  CampaignType = "BRAND_SEARCH"
	CampaignPlace        CampaignType = "PLACE"
	CampaignPowerContent CampaignType = "POWER_CONTENTS"
)

type DeliveryMethod string

const (
	DeliveryAccelerated DeliveryMethod = "ACCELERATED"
	DeliveryStandard    DeliveryMethod = "STANDARD"
)

type TrackingMode string

const (
	TrackingDisabled    TrackingMode = "TRACKING_DISABLED"
	TrackingAuto        TrackingMode = "AUTO_TRACKING_MODE"
	TrackingPassThrough TrackingMode = "PASS_THROUGH_SITE_MODE"
)

type EntityStatus string

const (
	StatusEligible        EntityStatus = "ELIGIBLE"
	StatusLimitedEligible EntityStatus = "LIMITED_ELIGIBLE"
	StatusPaused          EntityStatus = "PAUSED"
	StatusDeleted         EntityStatus = "DELETED"
)

type Direction string

const (
	DirectionNext     Direction = "NEXT"
	DirectionPrevious Direction = "PREVIOUS"
)

type ListOptions struct {
	Cursor    string
	Limit     int
	Direction Direction
}

type Page[T any] struct {
	Items          []T
	NextCursor     string
	PreviousCursor string
}

type Campaign struct {
	ID                         string         `json:"nccCampaignId"`
	CustomerID                 int64          `json:"customerId"`
	Type                       CampaignType   `json:"campaignTp"`
	Name                       string         `json:"name"`
	DailyBudget                int64          `json:"dailyBudget"`
	UseDailyBudget             bool           `json:"useDailyBudget"`
	DeliveryMethod             DeliveryMethod `json:"deliveryMethod"`
	UsePeriod                  bool           `json:"usePeriod"`
	PeriodStart                string         `json:"periodStartDt"`
	PeriodEnd                  string         `json:"periodEndDt"`
	TrackingMode               TrackingMode   `json:"trackingMode"`
	TrackingURL                string         `json:"trackingUrl"`
	TrackingURLCustomParams    string         `json:"trackingUrlCustomParams"`
	SharedBudgetID             string         `json:"sharedBudgetId"`
	SharedBudgetName           string         `json:"sharedBudgetName"`
	SharedDailyBudget          int64          `json:"sharedDailyBudget"`
	SharedBudgetExpectedCost   int64          `json:"sharedBudgetExpectCost"`
	SharedBudgetLock           bool           `json:"sharedBudgetLock"`
	SharedBudgetDeliveryMethod DeliveryMethod `json:"sharedBudgetDeliveryMethod"`
	UserLock                   bool           `json:"userLock"`
	Status                     EntityStatus   `json:"status"`
	StatusReason               string         `json:"statusReason"`
	NumberInUse                int            `json:"numberInUse"`
	CreatedAt                  string         `json:"regTm"`
	UpdatedAt                  string         `json:"editTm"`
}

type ListCampaignsRequest struct {
	Type CampaignType
	ListOptions
}

type CreateCampaignRequest struct {
	Name                    string
	Type                    CampaignType
	DailyBudget             *int64
	UseDailyBudget          *bool
	DeliveryMethod          DeliveryMethod
	UsePeriod               *bool
	PeriodStart             string
	PeriodEnd               string
	SharedBudgetID          string
	TrackingMode            TrackingMode
	TrackingURL             string
	TrackingURLCustomParams json.RawMessage
}

type CampaignBudgetUpdate struct {
	UseDailyBudget bool
	DailyBudget    int64
}

type CampaignPeriodUpdate struct {
	UsePeriod bool
	Start     string
	End       string
}

type AdGroupType string

const (
	AdGroupWebSite     AdGroupType = "WEB_SITE"
	AdGroupShopping    AdGroupType = "SHOPPING"
	AdGroupInformation AdGroupType = "INFORMATION"
	AdGroupProduct     AdGroupType = "PRODUCT"
	AdGroupBrandSearch AdGroupType = "BRAND_SEARCH"
	AdGroupPlace       AdGroupType = "PLACE"
	AdGroupCatalog     AdGroupType = "CATALOG"
)

type AdRollingType string

const (
	AdRollingRoundRobin  AdRollingType = "ROUND_ROBIN"
	AdRollingPerformance AdRollingType = "PERFORMANCE"
)

type AdGroup struct {
	ID                       string          `json:"nccAdgroupId"`
	CampaignID               string          `json:"nccCampaignId"`
	CustomerID               int64           `json:"customerId"`
	Name                     string          `json:"name"`
	Type                     AdGroupType     `json:"adgroupType"`
	PCChannelID              string          `json:"pcChannelId"`
	MobileChannelID          string          `json:"mobileChannelId"`
	BidAmount                int64           `json:"bidAmt"`
	DailyBudget              int64           `json:"dailyBudget"`
	UseDailyBudget           bool            `json:"useDailyBudget"`
	ContentsNetworkBidAmount int64           `json:"contentsNetworkBidAmt"`
	UseContentsNetworkBid    bool            `json:"useCntsNetworkBidAmt"`
	PCNetworkBidWeight       int             `json:"pcNetworkBidWeight"`
	MobileNetworkBidWeight   int             `json:"mobileNetworkBidWeight"`
	ContentsNetworkBidWeight int             `json:"contentsNetworkBidWeight"`
	UseContentsBidWeight     bool            `json:"useCntsNetworkBidWeight"`
	UseExpandedSearch        bool            `json:"useExpSearch"`
	ExpandedSearchBudget     int             `json:"expSearchBudgetRatio"`
	AIAdsOptIn               bool            `json:"aiAdsOptIn"`
	AdRollingType            AdRollingType   `json:"adRollingType"`
	Attributes               json.RawMessage `json:"adgroupAttrJson"`
	AutoBidStrategy          json.RawMessage `json:"autobidStrategy"`
	Targets                  json.RawMessage `json:"targets"`
	SharedBudgetID           string          `json:"sharedBudgetId"`
	UserLock                 bool            `json:"userLock"`
	BudgetLock               bool            `json:"budgetLock"`
	Status                   EntityStatus    `json:"status"`
	StatusReason             string          `json:"statusReason"`
	CreatedAt                string          `json:"regTm"`
	UpdatedAt                string          `json:"editTm"`
}

type ListAdGroupsRequest struct {
	CampaignID string
	ListOptions
}

type CreateAdGroupRequest struct {
	CampaignID               string
	Name                     string
	Type                     AdGroupType
	PCChannelID              string
	MobileChannelID          string
	BidAmount                *int64
	DailyBudget              *int64
	UseDailyBudget           *bool
	ContentsNetworkBidAmount *int64
	UseContentsNetworkBid    *bool
	AdRollingType            AdRollingType
	Attributes               json.RawMessage
	AutoBidStrategy          json.RawMessage
	Targets                  json.RawMessage
}

type AdGroupBudgetUpdate struct {
	UseDailyBudget bool
	DailyBudget    int64
}

type InspectStatus string

const (
	InspectUnderReview InspectStatus = "UNDER_REVIEW"
	InspectApproved    InspectStatus = "APPROVED"
	InspectEligible    InspectStatus = "ELIGIBLE"
	InspectPending     InspectStatus = "PENDING"
	InspectDenied      InspectStatus = "DENIED"
)

type Keyword struct {
	ID                 string          `json:"nccKeywordId"`
	AdGroupID          string          `json:"nccAdgroupId"`
	CampaignID         string          `json:"nccCampaignId"`
	CustomerID         int64           `json:"customerId"`
	Text               string          `json:"keyword"`
	BidAmount          int64           `json:"bidAmt"`
	UseGroupBidAmount  bool            `json:"useGroupBidAmt"`
	UserLock           bool            `json:"userLock"`
	Status             EntityStatus    `json:"status"`
	StatusReason       string          `json:"statusReason"`
	InspectStatus      InspectStatus   `json:"inspectStatus"`
	InspectRequest     string          `json:"inspectRequestMsg"`
	Links              json.RawMessage `json:"links"`
	Attributes         json.RawMessage `json:"attr"`
	Quality            json.RawMessage `json:"nccQi"`
	ResultStatus       *MutationStatus `json:"resultStatus"`
	AdRelevanceScore   int             `json:"adRelevanceScore"`
	ExpectedClickScore int             `json:"expectedClickScore"`
	CreatedAt          string          `json:"regTm"`
	UpdatedAt          string          `json:"editTm"`
}

type MutationStatus struct {
	Code int `json:"code"`
}

type ListKeywordsRequest struct {
	AdGroupID string
	ListOptions
}

type CreateKeywordRequest struct {
	Text              string
	UseGroupBidAmount bool
	BidAmount         *int64
	Links             json.RawMessage
	InspectRequest    string
}

type UpdateKeywordRequest struct {
	ID                string
	Paused            *bool
	UseGroupBidAmount *bool
	BidAmount         *int64
	Links             json.RawMessage
	InspectRequest    *string
}

// Date is a KST calendar date in YYYY-MM-DD form.
type Date string

type TimeRange struct {
	Since Date `json:"since"`
	Until Date `json:"until"`
}

type DatePreset string

const (
	DateToday       DatePreset = "today"
	DateYesterday   DatePreset = "yesterday"
	DateLast7Days   DatePreset = "last7days"
	DateLast30Days  DatePreset = "last30days"
	DateLastWeek    DatePreset = "lastweek"
	DateLastMonth   DatePreset = "lastmonth"
	DateLastQuarter DatePreset = "lastquarter"
)

type TimeIncrement string

const (
	TimeIncrementDaily   TimeIncrement = "1"
	TimeIncrementAllDays TimeIncrement = "allDays"
)

type Breakdown string

const (
	BreakdownDevice    Breakdown = "pcMblTp"
	BreakdownDayOfWeek Breakdown = "dayw"
	BreakdownHour      Breakdown = "hh24"
	BreakdownRegion    Breakdown = "regnNo"
)

type StatField string

const (
	StatImpressions              StatField = "impCnt"
	StatClicks                   StatField = "clkCnt"
	StatSpend                    StatField = "salesAmt"
	StatCTR                      StatField = "ctr"
	StatCPC                      StatField = "cpc"
	StatAverageRank              StatField = "avgRnk"
	StatConversions              StatField = "ccnt"
	StatRecentAverageRank        StatField = "recentAvgRnk"
	StatRecentAverageCPC         StatField = "recentAvgCpc"
	StatPCNetworkAverageRank     StatField = "pcNxAvgRnk"
	StatMobileNetworkAverageRank StatField = "mblNxAvgRnk"
	StatConversionRate           StatField = "crto"
	StatConversionAmount         StatField = "convAmt"
	StatReturnOnRevenue          StatField = "ror"
	StatCostPerConversion        StatField = "cpConv"
	StatVideoViews               StatField = "viewCnt"
	StatPurchaseConversions      StatField = "purchaseCcnt"
	StatPurchaseAmount           StatField = "purchaseConvAmt"
	StatPurchaseReturn           StatField = "purchaseRor"
)

type StatQuery struct {
	IDs           []string
	Fields        []StatField
	TimeRange     *TimeRange
	DatePreset    DatePreset
	TimeIncrement TimeIncrement
	Breakdown     Breakdown
}

// JSONValue preserves dynamic Stat values without converting numbers through
// float64.
type JSONValue struct{ raw json.RawMessage }

func (value *JSONValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || len(data) > 1<<20 || !json.Valid(data) {
		return fmt.Errorf("naversearchads: invalid JSON value")
	}
	value.raw = append(value.raw[:0], data...)
	return nil
}

func (value JSONValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value JSONValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value JSONValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}
func (value JSONValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("naversearchads: decode target and JSON value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type StatRow map[string]JSONValue

type DailyStatResponse struct {
	Data          []StatRow `json:"data"`
	Summary       TimeRange `json:"summary"`
	CycleBaseTime string    `json:"cycleBaseTm"`
}

type SummaryStatResponse struct {
	Data          []StatRow `json:"data"`
	CycleBaseTime string    `json:"cycleBaseTm"`
}

type StatResponse struct {
	Daily   *DailyStatResponse   `json:"dailyStatResponse,omitempty"`
	Summary *SummaryStatResponse `json:"summaryStatResponse,omitempty"`
}

type StatReportType string

const (
	ReportAd                              StatReportType = "AD"
	ReportAdDetail                        StatReportType = "AD_DETAIL"
	ReportAdConversion                    StatReportType = "AD_CONVERSION"
	ReportAdConversionDetail              StatReportType = "AD_CONVERSION_DETAIL"
	ReportAdExtension                     StatReportType = "ADEXTENSION"
	ReportAdExtensionConversion           StatReportType = "ADEXTENSION_CONVERSION"
	ReportExpandedKeyword                 StatReportType = "EXPKEYWORD"
	ReportShoppingKeywordDetail           StatReportType = "SHOPPINGKEYWORD_DETAIL"
	ReportShoppingKeywordConversionDetail StatReportType = "SHOPPINGKEYWORD_CONVERSION_DETAIL"
	ReportShoppingBrandProduct            StatReportType = "SHOPPINGBRANDPRODUCT"
	ReportShoppingBrandProductConversion  StatReportType = "SHOPPINGBRANDPRODUCT_CONVERSION"
	ReportCriterion                       StatReportType = "CRITERION"
	ReportCriterionConversion             StatReportType = "CRITERION_CONVERSION"
)

// StatReportDate is YYYYMMDD in requests. NAVER responses may use RFC3339.
type StatReportDate string

type StatReportStatus string

const (
	ReportRegistered  StatReportStatus = "REGIST"
	ReportRunning     StatReportStatus = "RUNNING"
	ReportBuilt       StatReportStatus = "BUILT"
	ReportNoData      StatReportStatus = "NONE"
	ReportError       StatReportStatus = "ERROR"
	ReportWaiting     StatReportStatus = "WAITING"
	ReportAggregating StatReportStatus = "AGGREGATING"
)

type CreateStatReportRequest struct {
	Type StatReportType
	Date StatReportDate
}

type StatReport struct {
	ID          int64            `json:"reportJobId"`
	Type        StatReportType   `json:"reportTp"`
	Status      StatReportStatus `json:"status"`
	DownloadURL string           `json:"downloadUrl"`
	Date        StatReportDate   `json:"statDt"`
	UpdatedAt   string           `json:"updateTm"`
}

type DownloadOptions struct {
	// MaxBytes bounds bytes written to Output. Zero uses DefaultMaxReportBytes.
	MaxBytes int64
}

type DownloadResult struct {
	Report       StatReport
	StatusCode   int
	BytesWritten int64
	ContentType  string
	ETag         string
	LastModified string
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (Page[Campaign], error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaignBudget(context.Context, string, CampaignBudgetUpdate, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaignPeriod(context.Context, string, CampaignPeriodUpdate, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignPaused(context.Context, string, bool, ...socialhub.CallOption) (*Campaign, error)
	DeleteCampaign(context.Context, string, ...socialhub.CallOption) error
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListAdGroupsRequest, ...socialhub.CallOption) (Page[AdGroup], error)
	GetAdGroup(context.Context, string, ...socialhub.CallOption) (*AdGroup, error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroupBid(context.Context, string, int64, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroupBudget(context.Context, string, AdGroupBudgetUpdate, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupPaused(context.Context, string, bool, ...socialhub.CallOption) (*AdGroup, error)
	DeleteAdGroup(context.Context, string, ...socialhub.CallOption) error
}

type KeywordWorkflow interface {
	ListKeywords(context.Context, ListKeywordsRequest, ...socialhub.CallOption) (Page[Keyword], error)
	GetKeyword(context.Context, string, ...socialhub.CallOption) (*Keyword, error)
	CreateKeywords(context.Context, string, []CreateKeywordRequest, ...socialhub.CallOption) ([]Keyword, error)
	UpdateKeywords(context.Context, []UpdateKeywordRequest, ...socialhub.CallOption) ([]Keyword, error)
	SetKeywordPaused(context.Context, string, bool, ...socialhub.CallOption) (*Keyword, error)
	DeleteKeyword(context.Context, string, ...socialhub.CallOption) error
}

type StatisticsWorkflow interface {
	Stats(context.Context, StatQuery, ...socialhub.CallOption) (StatResponse, error)
}

type StatReportWorkflow interface {
	ListStatReports(context.Context, ...socialhub.CallOption) ([]StatReport, error)
	CreateStatReport(context.Context, CreateStatReportRequest, ...socialhub.CallOption) (*StatReport, error)
	GetStatReport(context.Context, int64, ...socialhub.CallOption) (*StatReport, error)
	DeleteStatReport(context.Context, int64, ...socialhub.CallOption) error
	DeleteAllStatReports(context.Context, ...socialhub.CallOption) error
	DownloadStatReport(context.Context, int64, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
}

var (
	_ CampaignWorkflow   = (*Client)(nil)
	_ AdGroupWorkflow    = (*Client)(nil)
	_ KeywordWorkflow    = (*Client)(nil)
	_ StatisticsWorkflow = (*Client)(nil)
	_ StatReportWorkflow = (*Client)(nil)
)
