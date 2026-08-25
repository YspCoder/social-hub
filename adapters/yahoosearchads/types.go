package yahoosearchads

import (
	"context"
	"io"

	"social-hub/pkg/socialhub"
)

const (
	MaximumMutationBatch       = 2000
	MaximumSelectorIDs         = 1000
	MaximumPageSize            = int32(10_000)
	MaximumReportMutationBatch = 30
	MaximumReportPageSize      = int32(500)
	DefaultMaxReportBytes      = int64(256 << 20)
)

type PageRequest struct {
	StartIndex    int32 `json:"startIndex,omitempty"`
	NumberResults int32 `json:"numberResults,omitempty"`
}

type Page[T any] struct {
	Items           []T
	TotalNumEntries int32
	StartIndex      int32
	NumberResults   int32
	RID             string
}

func (page Page[T]) NextStartIndex() (int32, bool) {
	start := page.StartIndex
	if start == 0 {
		start = 1
	}
	next := start + int32(len(page.Items))
	return next, len(page.Items) > 0 && next <= page.TotalNumEntries
}

type ErrorDetail struct {
	RequestKey   string `json:"requestKey"`
	RequestValue string `json:"requestValue"`
}

type ErrorItem struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

type MutationItem[T any] struct {
	Value     *T
	Succeeded bool
	Errors    []ErrorItem
}

type MutationResult[T any] struct {
	Items []MutationItem[T]
	RID   string
}

type UserStatus string

const (
	StatusActive  UserStatus = "ACTIVE"
	StatusPaused  UserStatus = "PAUSED"
	StatusUnknown UserStatus = "UNKNOWN"
)

type CampaignType string

const (
	CampaignStandard CampaignType = "STANDARD"
	CampaignMobile   CampaignType = "MOBILE_APP"
	CampaignDynamic  CampaignType = "DYNAMIC_ADS_FOR_SEARCH"
)

type BiddingStrategyType string

const BiddingCPC BiddingStrategyType = "CPC"

type CampaignBudget struct {
	Amount           *int64 `json:"amount,omitempty"`
	CampaignBudgetID *int64 `json:"campaignBudgetId,omitempty"`
	Name             string `json:"campaignBudgetName,omitempty"`
}

type CampaignBiddingScheme struct {
	BiddingStrategyType BiddingStrategyType `json:"biddingStrategyType,omitempty"`
}

type CampaignBiddingStrategy struct {
	BiddingScheme      *CampaignBiddingScheme `json:"biddingScheme,omitempty"`
	PortfolioBiddingID *int64                 `json:"portfolioBiddingId,omitempty"`
}

type Campaign struct {
	AccountID                    int64                    `json:"accountId,omitempty"`
	CampaignID                   int64                    `json:"campaignId,omitempty"`
	CampaignName                 string                   `json:"campaignName,omitempty"`
	Type                         CampaignType             `json:"type,omitempty"`
	UserStatus                   UserStatus               `json:"userStatus,omitempty"`
	ServingStatus                string                   `json:"servingStatus,omitempty"`
	Budget                       *CampaignBudget          `json:"budget,omitempty"`
	BiddingStrategyConfiguration *CampaignBiddingStrategy `json:"biddingStrategyConfiguration,omitempty"`
	StartDate                    string                   `json:"startDate,omitempty"`
	EndDate                      string                   `json:"endDate,omitempty"`
	TrackingURL                  string                   `json:"trackingUrl,omitempty"`
	CreatedDate                  string                   `json:"createdDate,omitempty"`
	UpdatedDate                  string                   `json:"updatedDate,omitempty"`
}

type CampaignAdd struct {
	Name         string
	BudgetAmount int64
	StartDate    string
	EndDate      string
}

type CampaignUpdate struct {
	ID           int64
	Name         *string
	BudgetAmount *int64
	StartDate    *string
	EndDate      *string
}

type CampaignSelector struct {
	CampaignIDs  []int64      `json:"campaignIds,omitempty"`
	UserStatuses []UserStatus `json:"userStatuses,omitempty"`
	PageRequest
}

type AdGroupCPCScheme struct {
	CPC *int64 `json:"cpc,omitempty"`
}

type AdGroupBiddingScheme struct {
	CPC *AdGroupCPCScheme `json:"cpcBiddingScheme,omitempty"`
}

type AdGroupBiddingStrategy struct {
	BiddingScheme *AdGroupBiddingScheme `json:"biddingScheme,omitempty"`
}

type AdGroup struct {
	AccountID                    int64                   `json:"accountId,omitempty"`
	CampaignID                   int64                   `json:"campaignId,omitempty"`
	CampaignName                 string                  `json:"campaignName,omitempty"`
	AdGroupID                    int64                   `json:"adGroupId,omitempty"`
	AdGroupName                  string                  `json:"adGroupName,omitempty"`
	UserStatus                   UserStatus              `json:"userStatus,omitempty"`
	ServingStatus                string                  `json:"servingStatus,omitempty"`
	BiddingStrategyConfiguration *AdGroupBiddingStrategy `json:"biddingStrategyConfiguration,omitempty"`
	TrackingURL                  string                  `json:"trackingUrl,omitempty"`
	CreatedDate                  string                  `json:"createdDate,omitempty"`
	UpdatedDate                  string                  `json:"updatedDate,omitempty"`
}

type AdGroupAdd struct {
	Name string
	CPC  int64
}

type AdGroupUpdate struct {
	ID   int64
	Name *string
	CPC  *int64
}

type AdGroupSelector struct {
	CampaignIDs  []int64      `json:"campaignIds,omitempty"`
	AdGroupIDs   []int64      `json:"adGroupIds,omitempty"`
	UserStatuses []UserStatus `json:"userStatuses,omitempty"`
	PageRequest
}

type KeywordMatchType string

const (
	MatchExact  KeywordMatchType = "EXACT"
	MatchPhrase KeywordMatchType = "PHRASE"
	MatchBroad  KeywordMatchType = "BROAD"
)

type CriterionUse string

const (
	CriterionBiddable CriterionUse = "BIDDABLE"
	CriterionNegative CriterionUse = "NEGATIVE"
)

type KeywordText struct {
	Text      string           `json:"text,omitempty"`
	MatchType KeywordMatchType `json:"keywordMatchType,omitempty"`
}

type Criterion struct {
	ID      int64        `json:"criterionId,omitempty"`
	Keyword *KeywordText `json:"keyword,omitempty"`
}

type KeywordBid struct {
	AdGroupCPC int64  `json:"adGroupCpc,omitempty"`
	KeywordCPC int64  `json:"keywordCpc,omitempty"`
	CPC        *int64 `json:"cpc,omitempty"`
}

type BiddableKeyword struct {
	Bid         *KeywordBid `json:"bid,omitempty"`
	UserStatus  UserStatus  `json:"userStatus,omitempty"`
	FinalURL    string      `json:"finalUrl,omitempty"`
	TrackingURL string      `json:"trackingUrl,omitempty"`
}

type Keyword struct {
	AccountID         int64            `json:"accountId,omitempty"`
	CampaignID        int64            `json:"campaignId,omitempty"`
	CampaignName      string           `json:"campaignName,omitempty"`
	AdGroupID         int64            `json:"adGroupId,omitempty"`
	AdGroupName       string           `json:"adGroupName,omitempty"`
	Criterion         Criterion        `json:"criterion,omitempty"`
	Biddable          *BiddableKeyword `json:"biddableAdGroupCriterion,omitempty"`
	Use               CriterionUse     `json:"use,omitempty"`
	TrademarkStatus   string           `json:"trademarkStatus,omitempty"`
	InvalidTrademarks []string         `json:"invalidedTrademarks,omitempty"`
}

func (keyword Keyword) ID() int64 { return keyword.Criterion.ID }

type KeywordAdd struct {
	Text      string
	MatchType KeywordMatchType
	CPC       int64
}

type KeywordUpdate struct {
	ID  int64
	CPC *int64
}

type KeywordSelector struct {
	CampaignIDs  []int64      `json:"campaignIds,omitempty"`
	AdGroupIDs   []int64      `json:"adGroupIds,omitempty"`
	CriterionIDs []int64      `json:"criterionIds,omitempty"`
	UserStatuses []UserStatus `json:"userStatuses,omitempty"`
	Use          CriterionUse `json:"use"`
	PageRequest
}

type ReportType string

const (
	ReportAccount     ReportType = "ACCOUNT"
	ReportCampaign    ReportType = "CAMPAIGN"
	ReportAdGroup     ReportType = "ADGROUP"
	ReportAd          ReportType = "AD"
	ReportKeywords    ReportType = "KEYWORDS"
	ReportSearchQuery ReportType = "SEARCH_QUERY"
)

type ReportDateRangeType string

const (
	ReportToday      ReportDateRangeType = "TODAY"
	ReportYesterday  ReportDateRangeType = "YESTERDAY"
	ReportLast7Days  ReportDateRangeType = "LAST_7_DAYS"
	ReportLast30Days ReportDateRangeType = "LAST_30_DAYS"
	ReportThisMonth  ReportDateRangeType = "THIS_MONTH"
	ReportLastMonth  ReportDateRangeType = "LAST_MONTH"
	ReportAllTime    ReportDateRangeType = "ALL_TIME"
	ReportCustomDate ReportDateRangeType = "CUSTOM_DATE"
)

type ReportFormat string

const (
	ReportCSV ReportFormat = "CSV"
	ReportTSV ReportFormat = "TSV"
	ReportXML ReportFormat = "XML"
)

type ReportJobStatus string

const (
	ReportWaiting    ReportJobStatus = "WAIT"
	ReportCompleted  ReportJobStatus = "COMPLETED"
	ReportInProgress ReportJobStatus = "IN_PROGRESS"
	ReportFailed     ReportJobStatus = "FAILED"
	ReportUnknown    ReportJobStatus = "UNKNOWN"
)

type ReportDateRange struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

type ReportDefinition struct {
	AccountID               int64               `json:"accountId,omitempty"`
	CompleteTime            string              `json:"completeTime,omitempty"`
	DateRange               *ReportDateRange    `json:"dateRange,omitempty"`
	Fields                  []string            `json:"fields,omitempty"`
	ReportCompressType      string              `json:"reportCompressType,omitempty"`
	ReportDateRangeType     ReportDateRangeType `json:"reportDateRangeType,omitempty"`
	ReportDownloadEncode    string              `json:"reportDownloadEncode,omitempty"`
	ReportDownloadFormat    ReportFormat        `json:"reportDownloadFormat,omitempty"`
	ReportJobErrorDetail    string              `json:"reportJobErrorDetail,omitempty"`
	ReportJobID             int64               `json:"reportJobId,omitempty"`
	ReportJobStatus         ReportJobStatus     `json:"reportJobStatus,omitempty"`
	ReportLanguage          string              `json:"reportLanguage,omitempty"`
	ReportName              string              `json:"reportName,omitempty"`
	ReportSkipColumnHeader  string              `json:"reportSkipColumnHeader,omitempty"`
	ReportSkipReportSummary string              `json:"reportSkipReportSummary,omitempty"`
	ReportType              ReportType          `json:"reportType,omitempty"`
	RequestTime             string              `json:"requestTime,omitempty"`
}

type ReportDefinitionAdd struct {
	Name          string
	Type          ReportType
	Fields        []string
	DateRangeType ReportDateRangeType
	DateRange     *ReportDateRange
	Format        ReportFormat
	SkipHeader    bool
	SkipSummary   bool
}

type ReportSelector struct {
	ReportJobIDs      []int64           `json:"reportJobIds,omitempty"`
	ReportJobStatuses []ReportJobStatus `json:"reportJobStatuses,omitempty"`
	ReportTypes       []ReportType      `json:"reportTypes,omitempty"`
	PageRequest
}

type DownloadResult struct {
	RID          string
	HTTPStatus   int
	ContentType  string
	BytesWritten int64
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, CampaignSelector, ...socialhub.CallOption) (Page[Campaign], error)
	GetCampaign(context.Context, int64, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CampaignAdd, ...socialhub.CallOption) (*Campaign, MutationResult[Campaign], error)
	UpdateCampaigns(context.Context, []CampaignUpdate, ...socialhub.CallOption) (MutationResult[Campaign], error)
	SetCampaignsEnabled(context.Context, []int64, bool, ...socialhub.CallOption) (MutationResult[Campaign], error)
	DeleteCampaigns(context.Context, []int64, ...socialhub.CallOption) (MutationResult[Campaign], error)
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, AdGroupSelector, ...socialhub.CallOption) (Page[AdGroup], error)
	GetAdGroup(context.Context, int64, int64, ...socialhub.CallOption) (*AdGroup, error)
	CreateAdGroups(context.Context, int64, []AdGroupAdd, ...socialhub.CallOption) (MutationResult[AdGroup], error)
	UpdateAdGroups(context.Context, int64, []AdGroupUpdate, ...socialhub.CallOption) (MutationResult[AdGroup], error)
	SetAdGroupsEnabled(context.Context, int64, []int64, bool, ...socialhub.CallOption) (MutationResult[AdGroup], error)
	DeleteAdGroups(context.Context, int64, []int64, ...socialhub.CallOption) (MutationResult[AdGroup], error)
}

type KeywordWorkflow interface {
	ListKeywords(context.Context, KeywordSelector, ...socialhub.CallOption) (Page[Keyword], error)
	GetKeyword(context.Context, int64, int64, int64, ...socialhub.CallOption) (*Keyword, error)
	CreateKeywords(context.Context, int64, int64, []KeywordAdd, ...socialhub.CallOption) (MutationResult[Keyword], error)
	UpdateKeywords(context.Context, int64, int64, []KeywordUpdate, ...socialhub.CallOption) (MutationResult[Keyword], error)
	SetKeywordsEnabled(context.Context, int64, int64, []int64, bool, ...socialhub.CallOption) (MutationResult[Keyword], error)
	DeleteKeywords(context.Context, int64, int64, []int64, ...socialhub.CallOption) (MutationResult[Keyword], error)
}

type ReportWorkflow interface {
	CreateReport(context.Context, ReportDefinitionAdd, ...socialhub.CallOption) (*ReportDefinition, MutationResult[ReportDefinition], error)
	ListReports(context.Context, ReportSelector, ...socialhub.CallOption) (Page[ReportDefinition], error)
	GetReport(context.Context, int64, ...socialhub.CallOption) (*ReportDefinition, error)
	DownloadReport(context.Context, int64, io.Writer, int64, ...socialhub.CallOption) (DownloadResult, error)
	DeleteReports(context.Context, []int64, ...socialhub.CallOption) (MutationResult[ReportDefinition], error)
}
