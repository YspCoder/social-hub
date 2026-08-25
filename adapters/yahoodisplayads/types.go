package yahoodisplayads

import (
	"context"
	"io"

	"social-hub/pkg/socialhub"
)

const (
	MaximumCampaignMutationBatch = 300
	MaximumCampaignSelectorIDs   = 2000
	MaximumCampaignPageSize      = int32(2000)
	MaximumAdGroupMutationBatch  = 2000
	MaximumAdGroupSelectorIDs    = 1000
	MaximumAdGroupPageSize       = int32(10_000)
	MaximumAdMutationBatch       = 2000
	MaximumAdSelectorIDs         = 1000
	MaximumAdPageSize            = int32(10_000)
	MaximumReportMutationBatch   = 30
	MaximumReportPageSize        = int32(500)
	DefaultMaxReportBytes        = int64(256 << 20)
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

type BiddingStrategyType string

const (
	BiddingCPC                     BiddingStrategyType = "CPC"
	BiddingMaximizeConversions     BiddingStrategyType = "MAXIMIZE_CONVERSIONS"
	BiddingMaximizeConversionValue BiddingStrategyType = "MAXIMIZE_CONVERSION_VALUE"
	BiddingMaximizeClicks          BiddingStrategyType = "MAXIMIZE_CLICKS"
	BiddingMaximizeVideoViews      BiddingStrategyType = "MAXIMIZE_VIDEO_VIEWS"
	BiddingMaximizeViewable        BiddingStrategyType = "MAXIMIZE_VIEWABLE_IMPRESSIONS"
	BiddingVCPM                    BiddingStrategyType = "VCPM"
	BiddingCPV                     BiddingStrategyType = "CPV"
	BiddingCPF                     BiddingStrategyType = "CPF"
	BiddingUnknown                 BiddingStrategyType = "UNKNOWN"
)

const CampaignGoalWebsiteTraffic = "WEBSITE_TRAFFIC"

type CampaignBudget struct {
	Amount           *int64 `json:"amount,omitempty"`
	CampaignBudgetID *int64 `json:"campaignBudgetId,omitempty"`
	Name             string `json:"campaignBudgetName,omitempty"`
}

type CampaignCPCScheme struct {
	CPC                *int64 `json:"cpc,omitempty"`
	EnhancedCPCEnabled string `json:"enhancedCpcEnabled,omitempty"`
}

type CampaignBiddingScheme struct {
	BiddingStrategyType BiddingStrategyType `json:"biddingStrategyType,omitempty"`
	CPC                 *CampaignCPCScheme  `json:"cpcBiddingScheme,omitempty"`
}

type CampaignBiddingStrategy struct {
	BiddingScheme      *CampaignBiddingScheme `json:"biddingScheme,omitempty"`
	PortfolioBiddingID *int64                 `json:"portfolioBiddingId,omitempty"`
}

type Campaign struct {
	AccountID                    int64                    `json:"accountId,omitempty"`
	CampaignID                   int64                    `json:"campaignId,omitempty"`
	CampaignName                 string                   `json:"campaignName,omitempty"`
	CampaignGoal                 string                   `json:"campaignGoal,omitempty"`
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
	Goal         string
	BudgetAmount int64
	CPC          int64
	StartDate    string
	EndDate      string
}

type CampaignUpdate struct {
	ID           int64
	Name         *string
	BudgetAmount *int64
	CPC          *int64
	StartDate    *string
	EndDate      *string
}

type CampaignSelector struct {
	CampaignIDs  []int64      `json:"campaignIds,omitempty"`
	UserStatuses []UserStatus `json:"userStatuses,omitempty"`
	PageRequest
}

type DeviceType string
type DeviceAppType string
type DeviceOSType string

const (
	DeviceDesktop    DeviceType = "DESKTOP"
	DeviceSmartphone DeviceType = "SMARTPHONE"
	DeviceTablet     DeviceType = "TABLET"
	DeviceNone       DeviceType = "NONE"

	DeviceAppApp  DeviceAppType = "APP"
	DeviceAppWeb  DeviceAppType = "WEB"
	DeviceAppNone DeviceAppType = "NONE"

	DeviceOSIOS     DeviceOSType = "IOS"
	DeviceOSAndroid DeviceOSType = "ANDROID"
	DeviceOSNone    DeviceOSType = "NONE"
)

type AdGroupCPCScheme struct {
	CPC                *int64 `json:"cpc,omitempty"`
	EnhancedCPCEnabled string `json:"enhancedCpcEnabled,omitempty"`
}

type AdGroupBiddingScheme struct {
	CampaignBiddingStrategyType BiddingStrategyType `json:"campaignBiddingStrategyType,omitempty"`
	CPC                         *AdGroupCPCScheme   `json:"cpcBiddingScheme,omitempty"`
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
	BiddingStrategyConfiguration *AdGroupBiddingStrategy `json:"biddingStrategyConfiguration,omitempty"`
	Device                       []DeviceType            `json:"device,omitempty"`
	DeviceApp                    []DeviceAppType         `json:"deviceApp,omitempty"`
	DeviceOS                     []DeviceOSType          `json:"deviceOs,omitempty"`
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

type AdType string
type MainMediaFormat string
type ApprovalStatus string

const (
	AdTypeBanner AdType = "BANNER_AD"

	MediaFormatImage MainMediaFormat = "IMAGE"
	MediaFormatVideo MainMediaFormat = "VIDEO"
	MediaFormatNone  MainMediaFormat = "NONE"

	ApprovalApproved           ApprovalStatus = "APPROVED"
	ApprovalApprovedWithReview ApprovalStatus = "APPROVED_WITH_REVIEW"
	ApprovalReview             ApprovalStatus = "REVIEW"
	ApprovalPreDisapproved     ApprovalStatus = "PRE_DISAPPROVED"
	ApprovalPostDisapproved    ApprovalStatus = "POST_DISAPPROVED"
	ApprovalUnknown            ApprovalStatus = "UNKNOWN"
)

type BannerAd struct{}

type AdCreative struct {
	AdType          AdType          `json:"adType,omitempty"`
	MainMediaFormat MainMediaFormat `json:"mainMediaFormat,omitempty"`
	BannerAd        *BannerAd       `json:"bannerAd,omitempty"`
	DisplayURL      string          `json:"displayUrl,omitempty"`
	FinalURL        string          `json:"finalUrl,omitempty"`
	SmartphoneURL   string          `json:"smartphoneFinalUrl,omitempty"`
	TrackingURL     string          `json:"trackingUrl,omitempty"`
}

type Ad struct {
	AccountID                int64          `json:"accountId,omitempty"`
	CampaignID               int64          `json:"campaignId,omitempty"`
	CampaignName             string         `json:"campaignName,omitempty"`
	AdGroupID                int64          `json:"adGroupId,omitempty"`
	AdGroupName              string         `json:"adGroupName,omitempty"`
	AdID                     int64          `json:"adId,omitempty"`
	AdName                   string         `json:"adName,omitempty"`
	Ad                       *AdCreative    `json:"ad,omitempty"`
	MediaID                  int64          `json:"mediaId,omitempty"`
	UserStatus               UserStatus     `json:"userStatus,omitempty"`
	ApprovalStatus           ApprovalStatus `json:"approvalStatus,omitempty"`
	DisapprovalReasonCodes   []string       `json:"disapprovalReasonCodes,omitempty"`
	DisapprovalReasonDetails string         `json:"disapprovalReasonDescription,omitempty"`
	CreatedDate              string         `json:"createdDate,omitempty"`
	UpdatedDate              string         `json:"updatedDate,omitempty"`
}

type BannerAdAdd struct {
	Name     string
	MediaID  int64
	FinalURL string
}

type AdUpdate struct {
	ID       int64
	Name     *string
	FinalURL *string
}

type AdSelector struct {
	CampaignIDs  []int64      `json:"campaignIds,omitempty"`
	AdGroupIDs   []int64      `json:"adGroupIds,omitempty"`
	AdIDs        []int64      `json:"adIds,omitempty"`
	UserStatuses []UserStatus `json:"userStatuses,omitempty"`
	PageRequest
}

type ReportType string

const (
	ReportAD                 ReportType = "AD"
	ReportConversionPath     ReportType = "CONVERSION_PATH"
	ReportCrossCampaignReach ReportType = "CROSS_CAMPAIGN_REACHES"
	ReportAudienceListTarget ReportType = "AUDIENCE_LIST_TARGET"
	ReportPlacementTarget    ReportType = "PLACEMENT_TARGET"
	ReportLabel              ReportType = "LABEL"
	ReportReach              ReportType = "REACH"
	ReportURL                ReportType = "URL"
	ReportModelComparison    ReportType = "MODEL_COMPARISON"
	ReportContentKeywordList ReportType = "CONTENT_KEYWORD_LIST"
	ReportApp                ReportType = "APP"
	ReportCampaignBudget     ReportType = "CAMPAIGN_BUDGET"
	ReportPortfolioBidding   ReportType = "PORTFOLIO_BIDDING"
)

type ReportDateRangeType string

const (
	ReportCustomDate           ReportDateRangeType = "CUSTOM_DATE"
	ReportToday                ReportDateRangeType = "TODAY"
	ReportYesterday            ReportDateRangeType = "YESTERDAY"
	ReportLast7Days            ReportDateRangeType = "LAST_7_DAYS"
	ReportLastWeek             ReportDateRangeType = "LAST_WEEK"
	ReportLastBusinessWeek     ReportDateRangeType = "LAST_BUSINESS_WEEK"
	ReportLast14Days           ReportDateRangeType = "LAST_14_DAYS"
	ReportLast30Days           ReportDateRangeType = "LAST_30_DAYS"
	ReportThisMonth            ReportDateRangeType = "THIS_MONTH"
	ReportThisMonthExceptToday ReportDateRangeType = "THIS_MONTH_EXCEPT_TODAY"
	ReportLastMonth            ReportDateRangeType = "LAST_MONTH"
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
	ReportInProgress ReportJobStatus = "IN_PROGRESS"
	ReportCompleted  ReportJobStatus = "COMPLETED"
	ReportCanceled   ReportJobStatus = "CANCELED"
	ReportFailed     ReportJobStatus = "FAILED"
	ReportUnknown    ReportJobStatus = "UNKNOWN"
)

type ReportDateRange struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

type ReportTypeCondition struct {
	Type ReportType `json:"reportType,omitempty"`
}

type ReportDefinition struct {
	AccountID               int64                `json:"accountId,omitempty"`
	CompleteTime            string               `json:"completeTime,omitempty"`
	DateRange               *ReportDateRange     `json:"dateRange,omitempty"`
	Fields                  []string             `json:"fields,omitempty"`
	ReportCompressType      string               `json:"reportCompressType,omitempty"`
	ReportDateRangeType     ReportDateRangeType  `json:"reportDateRangeType,omitempty"`
	ReportDownloadEncode    string               `json:"reportDownloadEncode,omitempty"`
	ReportDownloadFormat    ReportFormat         `json:"reportDownloadFormat,omitempty"`
	ReportIncludeDeleted    string               `json:"reportIncludeDeleted,omitempty"`
	ReportJobStatus         ReportJobStatus      `json:"reportJobStatus,omitempty"`
	ReportJobErrorDetail    string               `json:"reportJobErrorDetail,omitempty"`
	ReportJobID             int64                `json:"reportJobId,omitempty"`
	ReportLanguage          string               `json:"reportLanguage,omitempty"`
	ReportName              string               `json:"reportName,omitempty"`
	RequestTime             string               `json:"requestTime,omitempty"`
	ReportSkipColumnHeader  string               `json:"reportSkipColumnHeader,omitempty"`
	ReportSkipReportSummary string               `json:"reportSkipReportSummary,omitempty"`
	ReportTypeCondition     *ReportTypeCondition `json:"reportTypeCondition,omitempty"`
}

type ReportDefinitionAdd struct {
	Name           string
	Fields         []string
	DateRangeType  ReportDateRangeType
	DateRange      *ReportDateRange
	Format         ReportFormat
	SkipHeader     bool
	SkipSummary    bool
	ExcludeDeleted bool
}

type ReportSelector struct {
	ReportJobIDs      []int64           `json:"reportJobIds,omitempty"`
	ReportJobStatuses []ReportJobStatus `json:"reportJobStatuses,omitempty"`
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

type AdWorkflow interface {
	ListAds(context.Context, AdSelector, ...socialhub.CallOption) (Page[Ad], error)
	GetAd(context.Context, int64, int64, int64, ...socialhub.CallOption) (*Ad, error)
	CreateBannerAds(context.Context, int64, int64, []BannerAdAdd, ...socialhub.CallOption) (MutationResult[Ad], error)
	UpdateAds(context.Context, int64, int64, []AdUpdate, ...socialhub.CallOption) (MutationResult[Ad], error)
	SetAdsEnabled(context.Context, int64, int64, []int64, bool, ...socialhub.CallOption) (MutationResult[Ad], error)
	DeleteAds(context.Context, int64, int64, []int64, ...socialhub.CallOption) (MutationResult[Ad], error)
}

type ReportWorkflow interface {
	CreateReport(context.Context, ReportDefinitionAdd, ...socialhub.CallOption) (*ReportDefinition, MutationResult[ReportDefinition], error)
	ListReports(context.Context, ReportSelector, ...socialhub.CallOption) (Page[ReportDefinition], error)
	GetReport(context.Context, int64, ...socialhub.CallOption) (*ReportDefinition, error)
	DownloadReport(context.Context, int64, io.Writer, int64, ...socialhub.CallOption) (DownloadResult, error)
	DeleteReports(context.Context, []int64, ...socialhub.CallOption) (MutationResult[ReportDefinition], error)
}
