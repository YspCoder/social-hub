package ads

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type ConfiguredStatus string

const (
	StatusActive   ConfiguredStatus = "ACTIVE"
	StatusArchived ConfiguredStatus = "ARCHIVED"
	StatusDeleted  ConfiguredStatus = "DELETED"
	StatusPaused   ConfiguredStatus = "PAUSED"
)

// CampaignObjective contains the objective values valid before Reddit's
// announced 2026-09-30 objective-enum migration.
type CampaignObjective string

const (
	ObjectiveAppInstalls              CampaignObjective = "APP_INSTALLS"
	ObjectiveCatalogSales             CampaignObjective = "CATALOG_SALES"
	ObjectiveClicks                   CampaignObjective = "CLICKS"
	ObjectiveConversions              CampaignObjective = "CONVERSIONS"
	ObjectiveImpressions              CampaignObjective = "IMPRESSIONS"
	ObjectiveLeadGeneration           CampaignObjective = "LEAD_GENERATION"
	ObjectiveVideoViewableImpressions CampaignObjective = "VIDEO_VIEWABLE_IMPRESSIONS"
)

type GoalType string

const (
	GoalDailySpend    GoalType = "DAILY_SPEND"
	GoalLifetimeSpend GoalType = "LIFETIME_SPEND"
)

type BidStrategy string

const (
	BidStrategyBidless        BidStrategy = "BIDLESS"
	BidStrategyManual         BidStrategy = "MANUAL_BIDDING"
	BidStrategyMaximizeVolume BidStrategy = "MAXIMIZE_VOLUME"
	BidStrategyTargetCPX      BidStrategy = "TARGET_CPX"
)

type BidType string

const (
	BidTypeCPC  BidType = "CPC"
	BidTypeCPM  BidType = "CPM"
	BidTypeCPV  BidType = "CPV"
	BidTypeCPV6 BidType = "CPV6"
)

type ReportBreakdown string

const (
	BreakdownAdAccountID ReportBreakdown = "AD_ACCOUNT_ID"
	BreakdownCampaignID  ReportBreakdown = "CAMPAIGN_ID"
	BreakdownAdGroupID   ReportBreakdown = "AD_GROUP_ID"
	BreakdownAdID        ReportBreakdown = "AD_ID"
	BreakdownDate        ReportBreakdown = "DATE"
	BreakdownHour        ReportBreakdown = "HOUR"
	BreakdownCountry     ReportBreakdown = "COUNTRY"
	BreakdownRegion      ReportBreakdown = "REGION"
	BreakdownPlacement   ReportBreakdown = "PLACEMENT"
)

// ReportField is open to newly added official fields. Constants cover common
// dimensions and delivery metrics without freezing Reddit's evolving enum.
type ReportField string

const (
	FieldAccountID   ReportField = "ACCOUNT_ID"
	FieldCampaignID  ReportField = "CAMPAIGN_ID"
	FieldAdGroupID   ReportField = "AD_GROUP_ID"
	FieldAdID        ReportField = "AD_ID"
	FieldDate        ReportField = "DATE"
	FieldHour        ReportField = "HOUR"
	FieldImpressions ReportField = "IMPRESSIONS"
	FieldClicks      ReportField = "CLICKS"
	FieldSpend       ReportField = "SPEND"
	FieldCPC         ReportField = "CPC"
	FieldCTR         ReportField = "CTR"
	FieldECPM        ReportField = "ECPM"
)

type AdAccount struct {
	ID            string `json:"id"`
	BusinessID    string `json:"business_id,omitempty"`
	Name          string `json:"name,omitempty"`
	Currency      string `json:"currency,omitempty"`
	TimeZoneID    string `json:"time_zone_id,omitempty"`
	AdminApproval string `json:"admin_approval,omitempty"`
	Type          string `json:"type,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	ModifiedAt    string `json:"modified_at,omitempty"`
}

type FundingInstrument struct {
	ID                 string     `json:"id"`
	AdAccountID        string     `json:"-"`
	Name               string     `json:"name,omitempty"`
	Currency           string     `json:"currency,omitempty"`
	CreditLimit        int64      `json:"credit_limit,omitempty"`
	BillableAmount     int64      `json:"billable_amount,omitempty"`
	StartTime          *time.Time `json:"start_time,omitempty"`
	EndTime            *time.Time `json:"end_time,omitempty"`
	IsServable         bool       `json:"is_servable,omitempty"`
	ReasonsNotServable []string   `json:"reasons_not_servable,omitempty"`
}

type Campaign struct {
	ID                           string            `json:"id"`
	AdAccountID                  string            `json:"ad_account_id,omitempty"`
	FundingInstrumentID          string            `json:"funding_instrument_id,omitempty"`
	Name                         string            `json:"name,omitempty"`
	ConfiguredStatus             ConfiguredStatus  `json:"configured_status,omitempty"`
	EffectiveStatus              string            `json:"effective_status,omitempty"`
	Objective                    CampaignObjective `json:"objective,omitempty"`
	IsCampaignBudgetOptimization *bool             `json:"is_campaign_budget_optimization,omitempty"`
	GoalType                     GoalType          `json:"goal_type,omitempty"`
	GoalValue                    *int64            `json:"goal_value,omitempty"`
	SpendCap                     *int64            `json:"spend_cap,omitempty"`
	StartTime                    string            `json:"start_time,omitempty"`
	EndTime                      string            `json:"end_time,omitempty"`
	BidStrategy                  BidStrategy       `json:"bid_strategy,omitempty"`
	BidType                      BidType           `json:"bid_type,omitempty"`
	BidValue                     *int64            `json:"bid_value,omitempty"`
	ConversionPixelID            string            `json:"conversion_pixel_id,omitempty"`
	CreatedAt                    string            `json:"created_at,omitempty"`
	ModifiedAt                   string            `json:"modified_at,omitempty"`
}

type AdGroup struct {
	ID                           string            `json:"id"`
	AdAccountID                  string            `json:"ad_account_id,omitempty"`
	CampaignID                   string            `json:"campaign_id"`
	Name                         string            `json:"name,omitempty"`
	ConfiguredStatus             ConfiguredStatus  `json:"configured_status,omitempty"`
	EffectiveStatus              string            `json:"effective_status,omitempty"`
	CampaignObjectiveType        CampaignObjective `json:"campaign_objective_type,omitempty"`
	IsCampaignBudgetOptimization *bool             `json:"is_campaign_budget_optimization,omitempty"`
	BidType                      BidType           `json:"bid_type,omitempty"`
	BidStrategy                  BidStrategy       `json:"bid_strategy,omitempty"`
	BidValue                     *int64            `json:"bid_value,omitempty"`
	GoalType                     GoalType          `json:"goal_type,omitempty"`
	GoalValue                    *int64            `json:"goal_value,omitempty"`
	StartTime                    string            `json:"start_time,omitempty"`
	EndTime                      string            `json:"end_time,omitempty"`
	ConversionPixelID            string            `json:"conversion_pixel_id,omitempty"`
	CreatedAt                    string            `json:"created_at,omitempty"`
	ModifiedAt                   string            `json:"modified_at,omitempty"`
}

type Ad struct {
	ID               string           `json:"id"`
	AdAccountID      string           `json:"ad_account_id,omitempty"`
	AdGroupID        string           `json:"ad_group_id"`
	CampaignID       string           `json:"campaign_id,omitempty"`
	Name             string           `json:"name,omitempty"`
	PostID           string           `json:"post_id,omitempty"`
	PostURL          string           `json:"post_url,omitempty"`
	ClickURL         string           `json:"click_url,omitempty"`
	ConfiguredStatus ConfiguredStatus `json:"configured_status,omitempty"`
	EffectiveStatus  string           `json:"effective_status,omitempty"`
	PreviewURL       string           `json:"preview_url,omitempty"`
	RejectionReason  string           `json:"rejection_reason,omitempty"`
	CreatedAt        string           `json:"created_at,omitempty"`
	ModifiedAt       string           `json:"modified_at,omitempty"`
}

type ListRequest struct {
	Cursor   string
	PageSize int
}

type CreateCampaignRequest struct {
	FundingInstrumentID          string
	Name                         string
	Objective                    CampaignObjective
	IsCampaignBudgetOptimization bool
	GoalType                     GoalType
	GoalValue                    *int64
	SpendCap                     *int64
	StartTime                    *time.Time
	EndTime                      *time.Time
	BidStrategy                  BidStrategy
	BidType                      BidType
	BidValue                     *int64
	ConversionPixelID            string
}

type UpdateCampaignRequest struct {
	Name   *string
	Status *ConfiguredStatus
}

type CreateAdGroupRequest struct {
	CampaignID        string
	Name              string
	BidType           BidType
	BidStrategy       *BidStrategy
	BidValue          *int64
	GoalType          GoalType
	GoalValue         *int64
	StartTime         time.Time
	EndTime           *time.Time
	ConversionPixelID string
}

type UpdateAdGroupRequest struct {
	Name   *string
	Status *ConfiguredStatus
}

type CreateAdRequest struct {
	AdGroupID string
	Name      string
	PostID    string
	ClickURL  string
}

type UpdateAdRequest struct {
	Name     *string
	Status   *ConfiguredStatus
	ClickURL *string
}

type ReportRequest struct {
	StartsAt   time.Time
	EndsAt     time.Time
	Fields     []ReportField
	Breakdowns []ReportBreakdown
	TimeZoneID string
	Filter     string
	Cursor     string
	PageSize   int
}

// ReportMetric preserves dynamic Reddit metric names, nulls, arrays, and
// integer precision.
type ReportMetric map[string]json.RawMessage

type ReportResult struct {
	Metrics          []ReportMetric
	MetricsUpdatedAt string
	NextCursor       *string
	HasMore          bool
	PageIndex        *int
	TotalCount       *int
}

type AccountWorkflow interface {
	GetAdAccount(context.Context, ...socialhub.CallOption) (*AdAccount, error)
}

type FundingWorkflow interface {
	ListFundingInstruments(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[FundingInstrument], error)
	GetFundingInstrument(context.Context, string, ...socialhub.CallOption) (*FundingInstrument, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[Campaign], error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[AdGroup], error)
	GetAdGroup(context.Context, string, ...socialhub.CallOption) (*AdGroup, error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, string, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
}

type AdWorkflow interface {
	ListAds(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[Ad], error)
	GetAd(context.Context, string, ...socialhub.CallOption) (*Ad, error)
	CreateAd(context.Context, CreateAdRequest, ...socialhub.CallOption) (*Ad, error)
	UpdateAd(context.Context, string, UpdateAdRequest, ...socialhub.CallOption) (*Ad, error)
}

type ReportWorkflow interface {
	GetReport(context.Context, ReportRequest, ...socialhub.CallOption) (ReportResult, error)
}

type singleResponse[T any] struct {
	Data T `json:"data"`
}

type pagination struct {
	NextURL     *string `json:"next_url"`
	PreviousURL *string `json:"previous_url"`
	PageIndex   *int    `json:"page_index,omitempty"`
	TotalCount  *int    `json:"total_count,omitempty"`
}

type listResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination pagination `json:"pagination"`
}
