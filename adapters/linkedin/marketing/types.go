package marketing

import (
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

// NumericID accepts LinkedIn's numeric JSON IDs while preserving exact digits.
type NumericID string

func (id *NumericID) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		if !validNumericID(text) {
			return fmt.Errorf("linkedin marketing: invalid numeric ID")
		}
		*id = NumericID(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil || !validNumericID(number.String()) {
		return fmt.Errorf("linkedin marketing: invalid numeric ID")
	}
	*id = NumericID(number.String())
	return nil
}

type Status string

const (
	StatusActive          Status = "ACTIVE"
	StatusPaused          Status = "PAUSED"
	StatusDraft           Status = "DRAFT"
	StatusArchived        Status = "ARCHIVED"
	StatusCompleted       Status = "COMPLETED"
	StatusCanceled        Status = "CANCELED"
	StatusPendingDeletion Status = "PENDING_DELETION"
	StatusRemoved         Status = "REMOVED"
)

type CostType string

const (
	CostCPC CostType = "CPC"
	CostCPM CostType = "CPM"
	CostCPV CostType = "CPV"
)

type ObjectiveType string

const (
	ObjectiveWebsiteVisit      ObjectiveType = "WEBSITE_VISITS"
	ObjectiveEngagement        ObjectiveType = "ENGAGEMENT"
	ObjectiveLeadGeneration    ObjectiveType = "LEAD_GENERATION"
	ObjectiveWebsiteConversion ObjectiveType = "WEBSITE_CONVERSIONS"
	ObjectiveBrandAwareness    ObjectiveType = "BRAND_AWARENESS"
	ObjectiveVideoView         ObjectiveType = "VIDEO_VIEWS"
	ObjectiveJobApplicant      ObjectiveType = "JOB_APPLICANTS"
)

type TimeGranularity string

const (
	GranularityAll     TimeGranularity = "ALL"
	GranularityDaily   TimeGranularity = "DAILY"
	GranularityMonthly TimeGranularity = "MONTHLY"
)

type AnalyticsPivot string

const (
	PivotAccount       AnalyticsPivot = "ACCOUNT"
	PivotCampaignGroup AnalyticsPivot = "CAMPAIGN_GROUP"
	PivotCampaign      AnalyticsPivot = "CAMPAIGN"
	PivotCreative      AnalyticsPivot = "CREATIVE"
)

type Money struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

type RunSchedule struct {
	Start int64 `json:"start"`
	End   int64 `json:"end,omitempty"`
}

type Locale struct {
	Language string `json:"language"`
	Country  string `json:"country"`
}

type TargetingClause struct {
	Or map[string][]string `json:"or"`
}

type TargetingConjunction struct {
	And []TargetingClause `json:"and"`
}

type TargetingCriteria struct {
	Include TargetingConjunction `json:"include"`
	Exclude *TargetingClause     `json:"exclude,omitempty"`
}

type AdAccount struct {
	ID       NumericID `json:"id"`
	Name     string    `json:"name,omitempty"`
	Currency string    `json:"currency,omitempty"`
	Type     string    `json:"type,omitempty"`
	Status   string    `json:"status,omitempty"`
	Test     bool      `json:"test,omitempty"`
}

type CampaignGroup struct {
	ID                   NumericID   `json:"id"`
	Account              string      `json:"account"`
	Name                 string      `json:"name,omitempty"`
	Status               Status      `json:"status,omitempty"`
	RunSchedule          RunSchedule `json:"runSchedule,omitempty"`
	TotalBudget          *Money      `json:"totalBudget,omitempty"`
	ServingStatuses      []string    `json:"servingStatuses,omitempty"`
	AllowedCampaignTypes []string    `json:"allowedCampaignTypes,omitempty"`
	Backfilled           bool        `json:"backfilled,omitempty"`
	Test                 bool        `json:"test,omitempty"`
}

type Campaign struct {
	ID                       NumericID         `json:"id"`
	Account                  string            `json:"account"`
	CampaignGroup            string            `json:"campaignGroup,omitempty"`
	AssociatedEntity         string            `json:"associatedEntity,omitempty"`
	Name                     string            `json:"name,omitempty"`
	Status                   Status            `json:"status,omitempty"`
	Type                     string            `json:"type,omitempty"`
	Objective                ObjectiveType     `json:"objectiveType,omitempty"`
	CostType                 CostType          `json:"costType,omitempty"`
	CreativeSelection        string            `json:"creativeSelection,omitempty"`
	DailyBudget              *Money            `json:"dailyBudget,omitempty"`
	TotalBudget              *Money            `json:"totalBudget,omitempty"`
	UnitCost                 *Money            `json:"unitCost,omitempty"`
	Locale                   Locale            `json:"locale,omitempty"`
	RunSchedule              RunSchedule       `json:"runSchedule,omitempty"`
	TargetingCriteria        TargetingCriteria `json:"targetingCriteria,omitempty"`
	AudienceExpansionEnabled bool              `json:"audienceExpansionEnabled,omitempty"`
	OffsiteDeliveryEnabled   bool              `json:"offsiteDeliveryEnabled,omitempty"`
	ServingStatuses          []string          `json:"servingStatuses,omitempty"`
	Test                     bool              `json:"test,omitempty"`
}

type CreativeContent struct {
	Reference string `json:"reference,omitempty"`
}

type Creative struct {
	ID                 string          `json:"id"`
	Account            string          `json:"account,omitempty"`
	Campaign           string          `json:"campaign"`
	Name               string          `json:"name,omitempty"`
	Content            CreativeContent `json:"content,omitempty"`
	InlineContent      json.RawMessage `json:"inlineContent,omitempty"`
	IntendedStatus     Status          `json:"intendedStatus,omitempty"`
	IsServing          bool            `json:"isServing,omitempty"`
	IsTest             bool            `json:"isTest,omitempty"`
	ServingHoldReasons []string        `json:"servingHoldReasons,omitempty"`
	CreatedAt          int64           `json:"createdAt,omitempty"`
	LastModifiedAt     int64           `json:"lastModifiedAt,omitempty"`
}

type ListRequest struct {
	Statuses   []Status
	Cursor     string
	MaxResults int
}

type CreateCampaignGroupRequest struct {
	Name        string
	RunSchedule RunSchedule
	TotalBudget *Money
}

type UpdateCampaignGroupRequest struct {
	Name        *string
	TotalBudget *Money
}

type CreateCampaignRequest struct {
	CampaignGroupID          string
	AssociatedEntityURN      string
	Name                     string
	Objective                ObjectiveType
	CostType                 CostType
	DailyBudget              Money
	TotalBudget              *Money
	UnitCost                 Money
	Locale                   Locale
	RunSchedule              RunSchedule
	TargetingCriteria        TargetingCriteria
	AudienceExpansionEnabled bool
	OffsiteDeliveryEnabled   bool
}

type UpdateCampaignRequest struct {
	Name        *string
	DailyBudget *Money
	TotalBudget *Money
	EndTime     *int64
}

type ListCreativesRequest struct {
	Cursor     string
	MaxResults int
}

type CreateCreativeRequest struct {
	CampaignID string
	ContentURN string
	Name       string
}

type AnalyticsRequest struct {
	StartDate   string
	EndDate     string
	Pivot       AnalyticsPivot
	Granularity TimeGranularity
	Fields      []string
}

type AnalyticsDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type AnalyticsDateRange struct {
	Start AnalyticsDate `json:"start"`
	End   AnalyticsDate `json:"end"`
}

type AnalyticsRow struct {
	DateRange   AnalyticsDateRange
	PivotValues []string
	Metrics     map[string]json.RawMessage
}

func (row *AnalyticsRow) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw := fields["dateRange"]; raw != nil {
		if err := json.Unmarshal(raw, &row.DateRange); err != nil {
			return err
		}
		delete(fields, "dateRange")
	}
	if raw := fields["pivotValues"]; raw != nil {
		if err := json.Unmarshal(raw, &row.PivotValues); err != nil {
			return err
		}
		delete(fields, "pivotValues")
	}
	row.Metrics = fields
	return nil
}

type AdAccountWorkflow interface {
	GetAdAccount(context.Context, ...socialhub.CallOption) (*AdAccount, error)
}

type CampaignGroupWorkflow interface {
	ListCampaignGroups(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[CampaignGroup], error)
	GetCampaignGroup(context.Context, string, ...socialhub.CallOption) (*CampaignGroup, error)
	CreateCampaignGroup(context.Context, CreateCampaignGroupRequest, ...socialhub.CallOption) (*CampaignGroup, error)
	UpdateCampaignGroup(context.Context, string, UpdateCampaignGroupRequest, ...socialhub.CallOption) (*CampaignGroup, error)
	SetCampaignGroupStatus(context.Context, string, Status, ...socialhub.CallOption) (*CampaignGroup, error)
	ArchiveCampaignGroup(context.Context, string, ...socialhub.CallOption) error
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[Campaign], error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignStatus(context.Context, string, Status, ...socialhub.CallOption) (*Campaign, error)
	ArchiveCampaign(context.Context, string, ...socialhub.CallOption) error
}

type CreativeWorkflow interface {
	ListCreatives(context.Context, ListCreativesRequest, ...socialhub.CallOption) (socialhub.Page[Creative], error)
	GetCreative(context.Context, string, ...socialhub.CallOption) (*Creative, error)
	CreateCreative(context.Context, CreateCreativeRequest, ...socialhub.CallOption) (*Creative, error)
	SetCreativeStatus(context.Context, string, Status, ...socialhub.CallOption) (*Creative, error)
	ArchiveCreative(context.Context, string, ...socialhub.CallOption) error
}

type AnalyticsWorkflow interface {
	GetAdAnalytics(context.Context, AnalyticsRequest, ...socialhub.CallOption) ([]AnalyticsRow, error)
}
