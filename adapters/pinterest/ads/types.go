package ads

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type EntityStatus string

const (
	StatusActive       EntityStatus = "ACTIVE"
	StatusPaused       EntityStatus = "PAUSED"
	StatusArchived     EntityStatus = "ARCHIVED"
	StatusDraft        EntityStatus = "DRAFT"
	StatusDeletedDraft EntityStatus = "DELETED_DRAFT"
)

type ObjectiveType string

const (
	ObjectiveAwareness        ObjectiveType = "AWARENESS"
	ObjectiveConsideration    ObjectiveType = "CONSIDERATION"
	ObjectiveWebConversion    ObjectiveType = "WEB_CONVERSION"
	ObjectiveCatalogSales     ObjectiveType = "CATALOG_SALES"
	ObjectiveVideoCompletion  ObjectiveType = "VIDEO_COMPLETION"
	ObjectiveAppInstall       ObjectiveType = "APP_INSTALL"
	ObjectiveSales            ObjectiveType = "SALES"
	ObjectiveLeads            ObjectiveType = "LEADS"
	ObjectiveCTVConsideration ObjectiveType = "CTV_CONSIDERATION"
)

type BillableEvent string

const (
	BillableClickthrough   BillableEvent = "CLICKTHROUGH"
	BillableImpression     BillableEvent = "IMPRESSION"
	BillableVideoView50MRC BillableEvent = "VIDEO_V_50_MRC"
)

type BudgetType string

const (
	BudgetDaily      BudgetType = "DAILY"
	BudgetLifetime   BudgetType = "LIFETIME"
	BudgetCBOAdGroup BudgetType = "CBO_ADGROUP"
)

type BidStrategyType string

const (
	BidAutomatic     BidStrategyType = "AUTOMATIC_BID"
	BidMaximum       BidStrategyType = "MAX_BID"
	BidTargetAverage BidStrategyType = "TARGET_AVG"
)

type PacingDeliveryType string

const (
	PacingStandard    PacingDeliveryType = "STANDARD"
	PacingAccelerated PacingDeliveryType = "ACCELERATED"
)

type PlacementGroup string

const (
	PlacementAll    PlacementGroup = "ALL"
	PlacementSearch PlacementGroup = "SEARCH"
	PlacementBrowse PlacementGroup = "BROWSE"
	PlacementOther  PlacementGroup = "OTHER"
)

type CreativeType string

const (
	CreativeRegular                   CreativeType = "REGULAR"
	CreativeVideo                     CreativeType = "VIDEO"
	CreativeShopping                  CreativeType = "SHOPPING"
	CreativeCarousel                  CreativeType = "CAROUSEL"
	CreativeMaxVideo                  CreativeType = "MAX_VIDEO"
	CreativeShopThePin                CreativeType = "SHOP_THE_PIN"
	CreativeCollection                CreativeType = "COLLECTION"
	CreativeIdea                      CreativeType = "IDEA"
	CreativeShowcase                  CreativeType = "SHOWCASE"
	CreativeQuiz                      CreativeType = "QUIZ"
	CreativeCollage                   CreativeType = "COLLAGE"
	CreativeMaxWidthRegularCollection CreativeType = "MAX_WIDTH_REGULAR_COLLECTION"
	CreativeMaxWidthVideoCollection   CreativeType = "MAX_WIDTH_VIDEO_COLLECTION"
	CreativeApp                       CreativeType = "APP"
)

type Granularity string

const (
	GranularityTotal Granularity = "TOTAL"
	GranularityDay   Granularity = "DAY"
	GranularityHour  Granularity = "HOUR"
	GranularityWeek  Granularity = "WEEK"
	GranularityMonth Granularity = "MONTH"
)

type ConversionReportTime string

const (
	ReportTimeAdAction   ConversionReportTime = "TIME_OF_AD_ACTION"
	ReportTimeConversion ConversionReportTime = "TIME_OF_CONVERSION"
)

type ReportingTimezone string

const (
	TimezonePinterest ReportingTimezone = "PINTEREST_TIME_ZONE"
	TimezoneAdAccount ReportingTimezone = "AD_ACCOUNT_TIME_ZONE"
)

type TargetingSpec map[string][]string

type AdAccount struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Country     string   `json:"country,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	TimeZone    string   `json:"time_zone,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	CreatedTime *int64   `json:"created_time,omitempty"`
	UpdatedTime *int64   `json:"updated_time,omitempty"`
}

type Campaign struct {
	ID                         string        `json:"id"`
	AdAccountID                string        `json:"ad_account_id,omitempty"`
	Name                       string        `json:"name,omitempty"`
	Objective                  ObjectiveType `json:"objective_type,omitempty"`
	Status                     EntityStatus  `json:"status,omitempty"`
	SummaryStatus              string        `json:"summary_status,omitempty"`
	DailySpendCap              *int64        `json:"daily_spend_cap,omitempty"`
	LifetimeSpendCap           *int64        `json:"lifetime_spend_cap,omitempty"`
	StartTime                  *int64        `json:"start_time,omitempty"`
	EndTime                    *int64        `json:"end_time,omitempty"`
	CampaignBudgetOptimization *bool         `json:"is_campaign_budget_optimization,omitempty"`
	FlexibleDailyBudgets       *bool         `json:"is_flexible_daily_budgets,omitempty"`
	PerformancePlus            bool          `json:"is_performance_plus,omitempty"`
	CreatedTime                int64         `json:"created_time,omitempty"`
	UpdatedTime                int64         `json:"updated_time,omitempty"`
}

type AdGroup struct {
	ID                       string                     `json:"id"`
	AdAccountID              string                     `json:"ad_account_id,omitempty"`
	CampaignID               string                     `json:"campaign_id,omitempty"`
	Name                     string                     `json:"name,omitempty"`
	Status                   EntityStatus               `json:"status,omitempty"`
	SummaryStatus            string                     `json:"summary_status,omitempty"`
	BillableEvent            BillableEvent              `json:"billable_event,omitempty"`
	BudgetType               BudgetType                 `json:"budget_type,omitempty"`
	BudgetInMicroCurrency    *int64                     `json:"budget_in_micro_currency,omitempty"`
	BidInMicroCurrency       *int64                     `json:"bid_in_micro_currency,omitempty"`
	BidStrategy              BidStrategyType            `json:"bid_strategy_type,omitempty"`
	Pacing                   PacingDeliveryType         `json:"pacing_delivery_type,omitempty"`
	Placement                PlacementGroup             `json:"placement_group,omitempty"`
	Targeting                TargetingSpec              `json:"targeting_spec,omitempty"`
	OptimizationGoalMetadata map[string]json.RawMessage `json:"optimization_goal_metadata,omitempty"`
	StartTime                *int64                     `json:"start_time,omitempty"`
	EndTime                  *int64                     `json:"end_time,omitempty"`
	CreatedTime              int64                      `json:"created_time,omitempty"`
	UpdatedTime              int64                      `json:"updated_time,omitempty"`
}

type Ad struct {
	ID               string       `json:"id"`
	AdAccountID      string       `json:"ad_account_id,omitempty"`
	CampaignID       string       `json:"campaign_id,omitempty"`
	AdGroupID        string       `json:"ad_group_id,omitempty"`
	PinID            string       `json:"pin_id,omitempty"`
	Name             string       `json:"name,omitempty"`
	CreativeType     CreativeType `json:"creative_type,omitempty"`
	Status           EntityStatus `json:"status,omitempty"`
	SummaryStatus    string       `json:"summary_status,omitempty"`
	ReviewStatus     string       `json:"review_status,omitempty"`
	DestinationURL   string       `json:"destination_url,omitempty"`
	ClickTrackingURL string       `json:"click_tracking_url,omitempty"`
	ViewTrackingURL  string       `json:"view_tracking_url,omitempty"`
	RejectedReasons  []string     `json:"rejected_reasons,omitempty"`
	RejectionLabels  []string     `json:"rejection_labels,omitempty"`
	CreatedTime      int64        `json:"created_time,omitempty"`
	UpdatedTime      int64        `json:"updated_time,omitempty"`
}

type ListAdAccountsRequest struct {
	IncludeSharedAccounts *bool
	Cursor                string
	MaxResults            int
}

type ListCampaignsRequest struct {
	IDs        []string
	Statuses   []EntityStatus
	Cursor     string
	MaxResults int
}

type CreateCampaignRequest struct {
	Name                       string
	Objective                  ObjectiveType
	CampaignBudgetOptimization bool
	DailySpendCap              int64
	LifetimeSpendCap           int64
	StartTime                  int64
	EndTime                    int64
}

type UpdateCampaignRequest struct {
	Name             *string
	DailySpendCap    *int64
	LifetimeSpendCap *int64
	StartTime        *int64
	EndTime          *int64
}

type ListAdGroupsRequest struct {
	IDs         []string
	CampaignIDs []string
	Statuses    []EntityStatus
	Cursor      string
	MaxResults  int
}

type CreateAdGroupRequest struct {
	CampaignID               string
	Name                     string
	BillableEvent            BillableEvent
	BudgetType               BudgetType
	BudgetInMicroCurrency    int64
	BidInMicroCurrency       int64
	BidStrategy              BidStrategyType
	Pacing                   PacingDeliveryType
	Placement                PlacementGroup
	Targeting                TargetingSpec
	OptimizationGoalMetadata map[string]any
	StartTime                int64
	EndTime                  int64
}

type UpdateAdGroupRequest struct {
	Name                     *string
	BudgetInMicroCurrency    *int64
	BidInMicroCurrency       *int64
	BidStrategy              *BidStrategyType
	Pacing                   *PacingDeliveryType
	Placement                *PlacementGroup
	Targeting                TargetingSpec
	OptimizationGoalMetadata map[string]any
	EndTime                  *int64
}

type ListAdsRequest struct {
	IDs         []string
	CampaignIDs []string
	AdGroupIDs  []string
	Statuses    []EntityStatus
	Cursor      string
	MaxResults  int
}

type CreateAdRequest struct {
	AdGroupID        string
	PinID            string
	Name             string
	CreativeType     CreativeType
	DestinationURL   string
	ClickTrackingURL string
	ViewTrackingURL  string
}

type UpdateAdRequest struct {
	Name             *string
	DestinationURL   *string
	ClickTrackingURL *string
	ViewTrackingURL  *string
}

type AnalyticsRequest struct {
	StartDate            string
	EndDate              string
	Columns              []string
	Granularity          Granularity
	ClickWindowDays      *int
	EngagementWindowDays *int
	ViewWindowDays       *int
	ConversionReportTime ConversionReportTime
	ReportingTimezone    ReportingTimezone
}

type AnalyticsRow struct {
	AdAccountID string
	Date        string
	Metrics     map[string]json.RawMessage
}

func (row *AnalyticsRow) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw := fields["AD_ACCOUNT_ID"]; raw != nil {
		if err := json.Unmarshal(raw, &row.AdAccountID); err != nil {
			return err
		}
		delete(fields, "AD_ACCOUNT_ID")
	}
	if raw := fields["DATE"]; raw != nil {
		if err := json.Unmarshal(raw, &row.Date); err != nil {
			return err
		}
		delete(fields, "DATE")
	}
	row.Metrics = fields
	return nil
}

type AdAccountWorkflow interface {
	ListAdAccounts(context.Context, ListAdAccountsRequest, ...socialhub.CallOption) (socialhub.Page[AdAccount], error)
	GetAdAccount(context.Context, ...socialhub.CallOption) (*AdAccount, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (socialhub.Page[Campaign], error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignStatus(context.Context, string, EntityStatus, ...socialhub.CallOption) (*Campaign, error)
	ArchiveCampaign(context.Context, string, ...socialhub.CallOption) error
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListAdGroupsRequest, ...socialhub.CallOption) (socialhub.Page[AdGroup], error)
	GetAdGroup(context.Context, string, ...socialhub.CallOption) (*AdGroup, error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, string, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupStatus(context.Context, string, EntityStatus, ...socialhub.CallOption) (*AdGroup, error)
	ArchiveAdGroup(context.Context, string, ...socialhub.CallOption) error
}

type AdWorkflow interface {
	ListAds(context.Context, ListAdsRequest, ...socialhub.CallOption) (socialhub.Page[Ad], error)
	GetAd(context.Context, string, ...socialhub.CallOption) (*Ad, error)
	CreateAd(context.Context, CreateAdRequest, ...socialhub.CallOption) (*Ad, error)
	UpdateAd(context.Context, string, UpdateAdRequest, ...socialhub.CallOption) (*Ad, error)
	SetAdStatus(context.Context, string, EntityStatus, ...socialhub.CallOption) (*Ad, error)
	ArchiveAd(context.Context, string, ...socialhub.CallOption) error
}

type AnalyticsWorkflow interface {
	GetAccountAnalytics(context.Context, AnalyticsRequest, ...socialhub.CallOption) ([]AnalyticsRow, error)
}
