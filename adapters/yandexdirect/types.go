package yandexdirect

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumCampaignMutationBatch = 10
	MaximumCampaignActionBatch   = 1000
	MaximumCampaignSelectionIDs  = 1000
	MaximumAdGroupMutationBatch  = 1000
	MaximumKeywordCreateBatch    = 200
	MaximumKeywordMutationBatch  = 1000
	MaximumKeywordActionBatch    = 10_000
	MaximumPageSize              = int64(10_000)
	DefaultMaxReportBytes        = int64(256 << 20)
)

type YesNo string

const (
	Yes YesNo = "YES"
	No  YesNo = "NO"
)

// Units describes Yandex's point accounting: spent by this request,
// currently available, and the advertiser or agency's daily limit.
type Units struct {
	Spent      int64
	Remaining  int64
	DailyLimit int64
}

// ResponseMetadata exposes Yandex headers required for support and adaptive
// point-based limiting.
type ResponseMetadata struct {
	RequestID      string
	Units          *Units
	UnitsUsedLogin string
}

type PageRequest struct {
	Limit  int64 `json:"Limit,omitempty"`
	Offset int64 `json:"Offset,omitempty"`
}

type Page[T any] struct {
	Items     []T
	LimitedBy *int64
	Metadata  ResponseMetadata
}

func (page Page[T]) NextOffset() (int64, bool) {
	if page.LimitedBy == nil {
		return 0, false
	}
	return *page.LimitedBy, true
}

type Notification struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
	Details string `json:"Details"`
}

type ActionResult struct {
	ID       int64          `json:"Id,omitempty"`
	Warnings []Notification `json:"Warnings,omitempty"`
	Errors   []Notification `json:"Errors,omitempty"`
}

type BatchResult struct {
	Items    []ActionResult
	Metadata ResponseMetadata
}

type StringArray struct {
	Items []string `json:"Items"`
}

type Int64Array struct {
	Items []int64 `json:"Items"`
}

// Date is a Yandex Direct calendar date in YYYY-MM-DD form.
type Date string

type CampaignType string

const (
	CampaignText    CampaignType = "TEXT_CAMPAIGN"
	CampaignUnified CampaignType = "UNIFIED_CAMPAIGN"
)

type CampaignState string

const (
	CampaignStateConverted CampaignState = "CONVERTED"
	CampaignStateArchived  CampaignState = "ARCHIVED"
	CampaignStateSuspended CampaignState = "SUSPENDED"
	CampaignStateEnded     CampaignState = "ENDED"
	CampaignStateOn        CampaignState = "ON"
	CampaignStateOff       CampaignState = "OFF"
	CampaignStateUnknown   CampaignState = "UNKNOWN"
)

type ModerationStatus string

const (
	StatusDraft       ModerationStatus = "DRAFT"
	StatusModeration  ModerationStatus = "MODERATION"
	StatusPreaccepted ModerationStatus = "PREACCEPTED"
	StatusAccepted    ModerationStatus = "ACCEPTED"
	StatusRejected    ModerationStatus = "REJECTED"
	StatusUnknown     ModerationStatus = "UNKNOWN"
)

type PaymentStatus string

const (
	PaymentAllowed    PaymentStatus = "ALLOWED"
	PaymentDisallowed PaymentStatus = "DISALLOWED"
)

type SearchStrategyType string

const SearchHighestPosition SearchStrategyType = "HIGHEST_POSITION"

type NetworkStrategyType string

const NetworkDefault NetworkStrategyType = "NETWORK_DEFAULT"

type SearchStrategy struct {
	BiddingStrategyType SearchStrategyType `json:"BiddingStrategyType"`
}

type NetworkDefaultSettings struct {
	LimitPercent int `json:"LimitPercent,omitempty"`
}

type NetworkStrategy struct {
	BiddingStrategyType NetworkStrategyType     `json:"BiddingStrategyType"`
	NetworkDefault      *NetworkDefaultSettings `json:"NetworkDefault,omitempty"`
}

type TextCampaignStrategy struct {
	Search  SearchStrategy  `json:"Search"`
	Network NetworkStrategy `json:"Network"`
}

type TextCampaignDetails struct {
	BiddingStrategy TextCampaignStrategy `json:"BiddingStrategy"`
}

type Campaign struct {
	ID                  int64                `json:"Id"`
	Name                string               `json:"Name"`
	StartDate           Date                 `json:"StartDate"`
	EndDate             Date                 `json:"EndDate"`
	TimeZone            string               `json:"TimeZone"`
	Type                CampaignType         `json:"Type"`
	State               CampaignState        `json:"State"`
	Status              ModerationStatus     `json:"Status"`
	StatusPayment       PaymentStatus        `json:"StatusPayment"`
	StatusClarification string               `json:"StatusClarification"`
	TextCampaign        *TextCampaignDetails `json:"TextCampaign,omitempty"`
}

type CampaignSelection struct {
	IDs             []int64            `json:"Ids,omitempty"`
	Types           []CampaignType     `json:"Types,omitempty"`
	States          []CampaignState    `json:"States,omitempty"`
	Statuses        []ModerationStatus `json:"Statuses,omitempty"`
	StatusesPayment []PaymentStatus    `json:"StatusesPayment,omitempty"`
}

type ListCampaignsRequest struct {
	Selection CampaignSelection
	Page      PageRequest
}

type CampaignUpdate struct {
	ID        int64  `json:"Id"`
	Name      string `json:"Name,omitempty"`
	StartDate Date   `json:"StartDate,omitempty"`
	EndDate   Date   `json:"EndDate,omitempty"`
}

type AdGroupType string

const (
	AdGroupText    AdGroupType = "TEXT_AD_GROUP"
	AdGroupUnified AdGroupType = "UNIFIED_AD_GROUP"
)

type ServingStatus string

const (
	ServingEligible     ServingStatus = "ELIGIBLE"
	ServingRarelyServed ServingStatus = "RARELY_SERVED"
)

type AdGroup struct {
	ID                          int64            `json:"Id"`
	Name                        string           `json:"Name"`
	CampaignID                  int64            `json:"CampaignId"`
	RegionIDs                   []int64          `json:"RegionIds"`
	RestrictedRegionIDs         *Int64Array      `json:"RestrictedRegionIds"`
	NegativeKeywords            *StringArray     `json:"NegativeKeywords"`
	NegativeKeywordSharedSetIDs *Int64Array      `json:"NegativeKeywordSharedSetIds"`
	TrackingParams              string           `json:"TrackingParams"`
	Status                      ModerationStatus `json:"Status"`
	ServingStatus               ServingStatus    `json:"ServingStatus"`
	Type                        AdGroupType      `json:"Type"`
}

type AdGroupSelection struct {
	CampaignIDs     []int64            `json:"CampaignIds,omitempty"`
	IDs             []int64            `json:"Ids,omitempty"`
	Types           []AdGroupType      `json:"Types,omitempty"`
	Statuses        []ModerationStatus `json:"Statuses,omitempty"`
	ServingStatuses []ServingStatus    `json:"ServingStatuses,omitempty"`
}

type ListAdGroupsRequest struct {
	Selection AdGroupSelection
	Page      PageRequest
}

type AdGroupAdd struct {
	Name                        string       `json:"Name"`
	RegionIDs                   []int64      `json:"RegionIds"`
	NegativeKeywords            *StringArray `json:"NegativeKeywords,omitempty"`
	NegativeKeywordSharedSetIDs *Int64Array  `json:"NegativeKeywordSharedSetIds,omitempty"`
	TrackingParams              string       `json:"TrackingParams,omitempty"`
}

type AdGroupUpdate struct {
	ID                          int64        `json:"Id"`
	Name                        string       `json:"Name,omitempty"`
	RegionIDs                   []int64      `json:"RegionIds,omitempty"`
	NegativeKeywords            *StringArray `json:"NegativeKeywords,omitempty"`
	NegativeKeywordSharedSetIDs *Int64Array  `json:"NegativeKeywordSharedSetIds,omitempty"`
	TrackingParams              string       `json:"TrackingParams,omitempty"`
}

type KeywordState string

const (
	KeywordStateOff       KeywordState = "OFF"
	KeywordStateOn        KeywordState = "ON"
	KeywordStateSuspended KeywordState = "SUSPENDED"
)

type StrategyPriority string

const (
	PriorityLow    StrategyPriority = "LOW"
	PriorityNormal StrategyPriority = "NORMAL"
	PriorityHigh   StrategyPriority = "HIGH"
)

type Keyword struct {
	ID                           int64            `json:"Id"`
	Keyword                      string           `json:"Keyword"`
	AdGroupID                    int64            `json:"AdGroupId"`
	CampaignID                   int64            `json:"CampaignId"`
	Bid                          int64            `json:"Bid"`
	AutotargetingSearchBidIsAuto YesNo            `json:"AutotargetingSearchBidIsAuto"`
	ContextBid                   int64            `json:"ContextBid"`
	StrategyPriority             StrategyPriority `json:"StrategyPriority"`
	UserParam1                   *string          `json:"UserParam1"`
	UserParam2                   *string          `json:"UserParam2"`
	State                        KeywordState     `json:"State"`
	Status                       ModerationStatus `json:"Status"`
	ServingStatus                ServingStatus    `json:"ServingStatus"`
}

type KeywordSelection struct {
	IDs             []int64            `json:"Ids,omitempty"`
	AdGroupIDs      []int64            `json:"AdGroupIds,omitempty"`
	CampaignIDs     []int64            `json:"CampaignIds,omitempty"`
	States          []KeywordState     `json:"States,omitempty"`
	Statuses        []ModerationStatus `json:"Statuses,omitempty"`
	ServingStatuses []ServingStatus    `json:"ServingStatuses,omitempty"`
	ModifiedSince   string             `json:"ModifiedSince,omitempty"`
}

type ListKeywordsRequest struct {
	Selection KeywordSelection
	Page      PageRequest
}

type KeywordAdd struct {
	Keyword                      string           `json:"Keyword"`
	Bid                          *int64           `json:"Bid,omitempty"`
	AutotargetingSearchBidIsAuto YesNo            `json:"AutotargetingSearchBidIsAuto,omitempty"`
	ContextBid                   *int64           `json:"ContextBid,omitempty"`
	StrategyPriority             StrategyPriority `json:"StrategyPriority,omitempty"`
	UserParam1                   string           `json:"UserParam1,omitempty"`
	UserParam2                   string           `json:"UserParam2,omitempty"`
}

type KeywordUpdate struct {
	ID                           int64             `json:"Id"`
	Keyword                      *string           `json:"Keyword,omitempty"`
	Bid                          *int64            `json:"Bid,omitempty"`
	AutotargetingSearchBidIsAuto *YesNo            `json:"AutotargetingSearchBidIsAuto,omitempty"`
	ContextBid                   *int64            `json:"ContextBid,omitempty"`
	StrategyPriority             *StrategyPriority `json:"StrategyPriority,omitempty"`
	UserParam1                   *string           `json:"UserParam1,omitempty"`
	UserParam2                   *string           `json:"UserParam2,omitempty"`
}

type ReportType string

const (
	ReportAccount        ReportType = "ACCOUNT_PERFORMANCE_REPORT"
	ReportCampaign       ReportType = "CAMPAIGN_PERFORMANCE_REPORT"
	ReportAdGroup        ReportType = "ADGROUP_PERFORMANCE_REPORT"
	ReportAd             ReportType = "AD_PERFORMANCE_REPORT"
	ReportCriteria       ReportType = "CRITERIA_PERFORMANCE_REPORT"
	ReportCustom         ReportType = "CUSTOM_REPORT"
	ReportReachFrequency ReportType = "REACH_AND_FREQUENCY_PERFORMANCE_REPORT"
	ReportSearchQuery    ReportType = "SEARCH_QUERY_PERFORMANCE_REPORT"
)

type DateRangeType string

const (
	DateRangeAllTime    DateRangeType = "ALL_TIME"
	DateRangeAuto       DateRangeType = "AUTO"
	DateRangeCustom     DateRangeType = "CUSTOM_DATE"
	DateRangeToday      DateRangeType = "TODAY"
	DateRangeYesterday  DateRangeType = "YESTERDAY"
	DateRangeLast7Days  DateRangeType = "LAST_7_DAYS"
	DateRangeLast30Days DateRangeType = "LAST_30_DAYS"
	DateRangeThisMonth  DateRangeType = "THIS_MONTH"
	DateRangeLastMonth  DateRangeType = "LAST_MONTH"
)

type ReportField string

const (
	FieldDate         ReportField = "Date"
	FieldCampaignID   ReportField = "CampaignId"
	FieldCampaignName ReportField = "CampaignName"
	FieldAdGroupID    ReportField = "AdGroupId"
	FieldAdGroupName  ReportField = "AdGroupName"
	FieldAdID         ReportField = "AdId"
	FieldCriterionID  ReportField = "CriterionId"
	FieldCriterion    ReportField = "Criterion"
	FieldQuery        ReportField = "Query"
	FieldImpressions  ReportField = "Impressions"
	FieldClicks       ReportField = "Clicks"
	FieldCost         ReportField = "Cost"
	FieldConversions  ReportField = "Conversions"
	FieldRevenue      ReportField = "Revenue"
)

type FilterOperator string

const (
	FilterEquals                  FilterOperator = "EQUALS"
	FilterNotEquals               FilterOperator = "NOT_EQUALS"
	FilterIn                      FilterOperator = "IN"
	FilterNotIn                   FilterOperator = "NOT_IN"
	FilterLessThan                FilterOperator = "LESS_THAN"
	FilterGreaterThan             FilterOperator = "GREATER_THAN"
	FilterStartsWithIgnoreCase    FilterOperator = "STARTS_WITH_IGNORE_CASE"
	FilterNotStartsWithIgnoreCase FilterOperator = "DOES_NOT_START_WITH_IGNORE_CASE"
)

type ReportFilter struct {
	Field    ReportField    `json:"Field"`
	Operator FilterOperator `json:"Operator"`
	Values   []string       `json:"Values"`
}

type ReportSelectionCriteria struct {
	DateFrom Date           `json:"DateFrom,omitempty"`
	DateTo   Date           `json:"DateTo,omitempty"`
	Filter   []ReportFilter `json:"Filter,omitempty"`
}

type ReportPage struct {
	Limit  int `json:"Limit"`
	Offset int `json:"Offset,omitempty"`
}

type SortOrder string

const (
	SortAscending  SortOrder = "ASCENDING"
	SortDescending SortOrder = "DESCENDING"
)

type ReportOrder struct {
	Field     ReportField `json:"Field"`
	SortOrder SortOrder   `json:"SortOrder,omitempty"`
}

type ReportDefinition struct {
	SelectionCriteria ReportSelectionCriteria `json:"SelectionCriteria"`
	Goals             []string                `json:"Goals,omitempty"`
	AttributionModels []string                `json:"AttributionModels,omitempty"`
	FieldNames        []ReportField           `json:"FieldNames"`
	Page              *ReportPage             `json:"Page,omitempty"`
	OrderBy           []ReportOrder           `json:"OrderBy,omitempty"`
	ReportName        string                  `json:"ReportName"`
	ReportType        ReportType              `json:"ReportType"`
	DateRangeType     DateRangeType           `json:"DateRangeType"`
	Format            string                  `json:"Format"`
	IncludeVAT        YesNo                   `json:"IncludeVAT"`
}

type ProcessingMode string

const (
	ProcessingAuto    ProcessingMode = "auto"
	ProcessingOnline  ProcessingMode = "online"
	ProcessingOffline ProcessingMode = "offline"
)

type ReportOptions struct {
	ProcessingMode    ProcessingMode
	SkipReportHeader  bool
	SkipColumnHeader  bool
	SkipReportSummary bool
	// MaxBytes bounds bytes written to output. Zero uses DefaultMaxReportBytes.
	MaxBytes int64
}

type ReportStatus string

const (
	ReportReady      ReportStatus = "READY"
	ReportQueued     ReportStatus = "QUEUED"
	ReportProcessing ReportStatus = "PROCESSING"
)

type ReportResult struct {
	Status         ReportStatus
	HTTPStatus     int
	RequestID      string
	RetryAfter     time.Duration
	ReportsInQueue int
	BytesWritten   int64
	ContentType    string
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (Page[Campaign], error)
	GetCampaign(context.Context, int64, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaigns(context.Context, []CampaignUpdate, ...socialhub.CallOption) (BatchResult, error)
	SuspendCampaigns(context.Context, []int64, ...socialhub.CallOption) (BatchResult, error)
	ResumeCampaigns(context.Context, []int64, ...socialhub.CallOption) (BatchResult, error)
	DeleteCampaigns(context.Context, []int64, ...socialhub.CallOption) (BatchResult, error)
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListAdGroupsRequest, ...socialhub.CallOption) (Page[AdGroup], error)
	GetAdGroup(context.Context, int64, ...socialhub.CallOption) (*AdGroup, error)
	CreateAdGroups(context.Context, int64, []AdGroupAdd, ...socialhub.CallOption) (BatchResult, error)
	UpdateAdGroups(context.Context, []AdGroupUpdate, ...socialhub.CallOption) (BatchResult, error)
	DeleteAdGroups(context.Context, []int64, ...socialhub.CallOption) (BatchResult, error)
}

type KeywordWorkflow interface {
	ListKeywords(context.Context, ListKeywordsRequest, ...socialhub.CallOption) (Page[Keyword], error)
	GetKeyword(context.Context, int64, ...socialhub.CallOption) (*Keyword, error)
	CreateKeywords(context.Context, int64, []KeywordAdd, ...socialhub.CallOption) (BatchResult, error)
	UpdateKeywords(context.Context, []KeywordUpdate, ...socialhub.CallOption) (BatchResult, error)
	SuspendKeywords(context.Context, []int64, ...socialhub.CallOption) (BatchResult, error)
	ResumeKeywords(context.Context, []int64, ...socialhub.CallOption) (BatchResult, error)
	DeleteKeywords(context.Context, []int64, ...socialhub.CallOption) (BatchResult, error)
}

type ReportWorkflow interface {
	GenerateReport(context.Context, ReportDefinition, io.Writer, ReportOptions, ...socialhub.CallOption) (ReportResult, error)
}

var (
	_ CampaignWorkflow = (*Client)(nil)
	_ AdGroupWorkflow  = (*Client)(nil)
	_ KeywordWorkflow  = (*Client)(nil)
	_ ReportWorkflow   = (*Client)(nil)
)
