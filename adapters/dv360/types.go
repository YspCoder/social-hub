package dv360

import (
	"context"

	"social-hub/pkg/socialhub"
)

type EntityStatus string

const (
	EntityStatusActive               EntityStatus = "ENTITY_STATUS_ACTIVE"
	EntityStatusArchived             EntityStatus = "ENTITY_STATUS_ARCHIVED"
	EntityStatusDraft                EntityStatus = "ENTITY_STATUS_DRAFT"
	EntityStatusPaused               EntityStatus = "ENTITY_STATUS_PAUSED"
	EntityStatusScheduledForDeletion EntityStatus = "ENTITY_STATUS_SCHEDULED_FOR_DELETION"
)

type EUPoliticalAdvertisingStatus string

const (
	ContainsEUPoliticalAdvertising       EUPoliticalAdvertisingStatus = "CONTAINS_EU_POLITICAL_ADVERTISING"
	DoesNotContainEUPoliticalAdvertising EUPoliticalAdvertisingStatus = "DOES_NOT_CONTAIN_EU_POLITICAL_ADVERTISING"
)

type CampaignGoalType string

const (
	CampaignGoalAppInstall     CampaignGoalType = "CAMPAIGN_GOAL_TYPE_APP_INSTALL"
	CampaignGoalBrandAwareness CampaignGoalType = "CAMPAIGN_GOAL_TYPE_BRAND_AWARENESS"
	CampaignGoalOfflineAction  CampaignGoalType = "CAMPAIGN_GOAL_TYPE_OFFLINE_ACTION"
	CampaignGoalOnlineAction   CampaignGoalType = "CAMPAIGN_GOAL_TYPE_ONLINE_ACTION"
)

type PerformanceGoalType string

const (
	PerformanceGoalCPM                 PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CPM"
	PerformanceGoalCPC                 PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CPC"
	PerformanceGoalCPA                 PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CPA"
	PerformanceGoalCTR                 PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CTR"
	PerformanceGoalViewability         PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_VIEWABILITY"
	PerformanceGoalCPIAVC              PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CPIAVC"
	PerformanceGoalCPE                 PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CPE"
	PerformanceGoalCPV                 PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CPV"
	PerformanceGoalClickCVR            PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_CLICK_CVR"
	PerformanceGoalImpressionCVR       PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_IMPRESSION_CVR"
	PerformanceGoalVCPM                PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_VCPM"
	PerformanceGoalVTR                 PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_VTR"
	PerformanceGoalAudioCompletionRate PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_AUDIO_COMPLETION_RATE"
	PerformanceGoalVideoCompletionRate PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_VIDEO_COMPLETION_RATE"
	PerformanceGoalOther               PerformanceGoalType = "PERFORMANCE_GOAL_TYPE_OTHER"
)

type TimeUnit string

const (
	TimeUnitMonths  TimeUnit = "TIME_UNIT_MONTHS"
	TimeUnitWeeks   TimeUnit = "TIME_UNIT_WEEKS"
	TimeUnitDays    TimeUnit = "TIME_UNIT_DAYS"
	TimeUnitHours   TimeUnit = "TIME_UNIT_HOURS"
	TimeUnitMinutes TimeUnit = "TIME_UNIT_MINUTES"
)

type BudgetUnit string

const (
	BudgetUnitCurrency    BudgetUnit = "BUDGET_UNIT_CURRENCY"
	BudgetUnitImpressions BudgetUnit = "BUDGET_UNIT_IMPRESSIONS"
)

type PacingType string

const (
	PacingAhead PacingType = "PACING_TYPE_AHEAD"
	PacingASAP  PacingType = "PACING_TYPE_ASAP"
	PacingEven  PacingType = "PACING_TYPE_EVEN"
)

type PacingPeriod string

const (
	PacingPeriodDaily  PacingPeriod = "PACING_PERIOD_DAILY"
	PacingPeriodFlight PacingPeriod = "PACING_PERIOD_FLIGHT"
)

type InsertionOrderType string

const (
	InsertionOrderRTB        InsertionOrderType = "RTB"
	InsertionOrderOverTheTop InsertionOrderType = "OVER_THE_TOP"
)

type InsertionOrderAutomationType string

const (
	InsertionOrderAutomationBudget    InsertionOrderAutomationType = "INSERTION_ORDER_AUTOMATION_TYPE_BUDGET"
	InsertionOrderAutomationNone      InsertionOrderAutomationType = "INSERTION_ORDER_AUTOMATION_TYPE_NONE"
	InsertionOrderAutomationBidBudget InsertionOrderAutomationType = "INSERTION_ORDER_AUTOMATION_TYPE_BID_BUDGET"
)

type OptimizationObjective string

const (
	OptimizationConversion     OptimizationObjective = "CONVERSION"
	OptimizationClick          OptimizationObjective = "CLICK"
	OptimizationBrandAwareness OptimizationObjective = "BRAND_AWARENESS"
	OptimizationCustom         OptimizationObjective = "CUSTOM"
	OptimizationNoObjective    OptimizationObjective = "NO_OBJECTIVE"
)

type KPIType string

const (
	KPICPM                 KPIType = "KPI_TYPE_CPM"
	KPICPC                 KPIType = "KPI_TYPE_CPC"
	KPICPA                 KPIType = "KPI_TYPE_CPA"
	KPICTR                 KPIType = "KPI_TYPE_CTR"
	KPIViewability         KPIType = "KPI_TYPE_VIEWABILITY"
	KPICPIAVC              KPIType = "KPI_TYPE_CPIAVC"
	KPICPE                 KPIType = "KPI_TYPE_CPE"
	KPICPV                 KPIType = "KPI_TYPE_CPV"
	KPIClickCVR            KPIType = "KPI_TYPE_CLICK_CVR"
	KPIImpressionCVR       KPIType = "KPI_TYPE_IMPRESSION_CVR"
	KPIVCPM                KPIType = "KPI_TYPE_VCPM"
	KPIVTR                 KPIType = "KPI_TYPE_VTR"
	KPIAudioCompletionRate KPIType = "KPI_TYPE_AUDIO_COMPLETION_RATE"
	KPIVideoCompletionRate KPIType = "KPI_TYPE_VIDEO_COMPLETION_RATE"
	KPICPCL                KPIType = "KPI_TYPE_CPCL"
	KPICPCV                KPIType = "KPI_TYPE_CPCV"
	KPITOS10               KPIType = "KPI_TYPE_TOS10"
	KPIMaximizePacing      KPIType = "KPI_TYPE_MAXIMIZE_PACING"
	KPICustomValueOverCost KPIType = "KPI_TYPE_CUSTOM_IMPRESSION_VALUE_OVER_COST"
	KPIOther               KPIType = "KPI_TYPE_OTHER"
)

type BiddingPerformanceGoalType string

const (
	BiddingGoalCPA         BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_CPA"
	BiddingGoalCPC         BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_CPC"
	BiddingGoalViewableCPM BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_VIEWABLE_CPM"
	BiddingGoalCustomAlgo  BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_CUSTOM_ALGO"
	BiddingGoalCIVA        BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_CIVA"
	BiddingGoalIVOTen      BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_IVO_TEN"
	BiddingGoalAVViewed    BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_AV_VIEWED"
	BiddingGoalReach       BiddingPerformanceGoalType = "BIDDING_STRATEGY_PERFORMANCE_GOAL_TYPE_REACH"
)

type LineItemType string

const (
	LineItemDisplayDefault LineItemType = "LINE_ITEM_TYPE_DISPLAY_DEFAULT"
	LineItemVideoDefault   LineItemType = "LINE_ITEM_TYPE_VIDEO_DEFAULT"
	LineItemAudioDefault   LineItemType = "LINE_ITEM_TYPE_AUDIO_DEFAULT"
)

type LineItemFlightDateType string

const (
	LineItemFlightInherited LineItemFlightDateType = "LINE_ITEM_FLIGHT_DATE_TYPE_INHERITED"
	LineItemFlightCustom    LineItemFlightDateType = "LINE_ITEM_FLIGHT_DATE_TYPE_CUSTOM"
)

type LineItemBudgetAllocationType string

const (
	LineItemBudgetAutomatic LineItemBudgetAllocationType = "LINE_ITEM_BUDGET_ALLOCATION_TYPE_AUTOMATIC"
	LineItemBudgetFixed     LineItemBudgetAllocationType = "LINE_ITEM_BUDGET_ALLOCATION_TYPE_FIXED"
	LineItemBudgetUnlimited LineItemBudgetAllocationType = "LINE_ITEM_BUDGET_ALLOCATION_TYPE_UNLIMITED"
)

type PartnerRevenueMarkupType string

const (
	PartnerRevenueCPM            PartnerRevenueMarkupType = "PARTNER_REVENUE_MODEL_MARKUP_TYPE_CPM"
	PartnerRevenueMediaCost      PartnerRevenueMarkupType = "PARTNER_REVENUE_MODEL_MARKUP_TYPE_MEDIA_COST_MARKUP"
	PartnerRevenueTotalMediaCost PartnerRevenueMarkupType = "PARTNER_REVENUE_MODEL_MARKUP_TYPE_TOTAL_MEDIA_COST_MARKUP"
)

// Date uses Google's whole-date JSON representation.
type Date struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type DateRange struct {
	StartDate Date  `json:"startDate"`
	EndDate   *Date `json:"endDate,omitempty"`
}

type FrequencyCap struct {
	Unlimited      bool     `json:"unlimited"`
	TimeUnit       TimeUnit `json:"timeUnit,omitempty"`
	TimeUnitCount  int      `json:"timeUnitCount,omitempty"`
	MaxImpressions int      `json:"maxImpressions,omitempty"`
}

type PerformanceGoal struct {
	Type             PerformanceGoalType `json:"performanceGoalType"`
	AmountMicros     string              `json:"performanceGoalAmountMicros,omitempty"`
	PercentageMicros string              `json:"performanceGoalPercentageMicros,omitempty"`
	Value            string              `json:"performanceGoalString,omitempty"`
}

type CampaignGoal struct {
	Type            CampaignGoalType `json:"campaignGoalType"`
	PerformanceGoal PerformanceGoal  `json:"performanceGoal"`
}

type CampaignFlight struct {
	PlannedSpendAmountMicros string    `json:"plannedSpendAmountMicros,omitempty"`
	PlannedDates             DateRange `json:"plannedDates"`
}

type CampaignBudget struct {
	ID                   string     `json:"budgetId,omitempty"`
	DisplayName          string     `json:"displayName"`
	BudgetAmountMicros   string     `json:"budgetAmountMicros"`
	DateRange            DateRange  `json:"dateRange"`
	ExternalBudgetSource string     `json:"externalBudgetSource"`
	BudgetUnit           BudgetUnit `json:"budgetUnit"`
	ExternalBudgetID     string     `json:"externalBudgetId,omitempty"`
	InvoiceGroupingID    string     `json:"invoiceGroupingId,omitempty"`
}

type Advertiser struct {
	Name                   string                       `json:"name"`
	AdvertiserID           string                       `json:"advertiserId"`
	PartnerID              string                       `json:"partnerId"`
	DisplayName            string                       `json:"displayName"`
	EntityStatus           EntityStatus                 `json:"entityStatus"`
	ContainsEUPoliticalAds EUPoliticalAdvertisingStatus `json:"containsEuPoliticalAds,omitempty"`
	UpdateTime             string                       `json:"updateTime,omitempty"`
}

type Campaign struct {
	Name            string           `json:"name"`
	AdvertiserID    string           `json:"advertiserId"`
	CampaignID      string           `json:"campaignId"`
	DisplayName     string           `json:"displayName"`
	EntityStatus    EntityStatus     `json:"entityStatus"`
	CampaignGoal    CampaignGoal     `json:"campaignGoal"`
	CampaignFlight  CampaignFlight   `json:"campaignFlight"`
	FrequencyCap    FrequencyCap     `json:"frequencyCap"`
	CampaignBudgets []CampaignBudget `json:"campaignBudgets,omitempty"`
	UpdateTime      string           `json:"updateTime,omitempty"`
}

type Pacing struct {
	Type                PacingType   `json:"pacingType"`
	Period              PacingPeriod `json:"pacingPeriod"`
	DailyMaxMicros      string       `json:"dailyMaxMicros,omitempty"`
	DailyMaxImpressions string       `json:"dailyMaxImpressions,omitempty"`
}

type InsertionOrderBudgetSegment struct {
	BudgetAmountMicros string    `json:"budgetAmountMicros"`
	DateRange          DateRange `json:"dateRange"`
	Description        string    `json:"description,omitempty"`
	CampaignBudgetID   string    `json:"campaignBudgetId,omitempty"`
}

type InsertionOrderBudget struct {
	BudgetUnit     BudgetUnit                    `json:"budgetUnit"`
	AutomationType InsertionOrderAutomationType  `json:"automationType,omitempty"`
	BudgetSegments []InsertionOrderBudgetSegment `json:"budgetSegments"`
}

type KPI struct {
	Type             KPIType `json:"kpiType"`
	AmountMicros     string  `json:"kpiAmountMicros,omitempty"`
	PercentageMicros string  `json:"kpiPercentageMicros,omitempty"`
	AlgorithmID      string  `json:"kpiAlgorithmId,omitempty"`
	Value            string  `json:"kpiString,omitempty"`
}

type FixedBidStrategy struct {
	BidAmountMicros string `json:"bidAmountMicros"`
}

type PerformanceGoalBidStrategy struct {
	Type                         BiddingPerformanceGoalType `json:"performanceGoalType"`
	AmountMicros                 string                     `json:"performanceGoalAmountMicros"`
	CustomBiddingAlgorithmID     string                     `json:"customBiddingAlgorithmId,omitempty"`
	MaxAverageCPMBidAmountMicros string                     `json:"maxAverageCpmBidAmountMicros,omitempty"`
}

type MaximizeSpendBidStrategy struct {
	Type                         BiddingPerformanceGoalType `json:"performanceGoalType"`
	CustomBiddingAlgorithmID     string                     `json:"customBiddingAlgorithmId,omitempty"`
	MaxAverageCPMBidAmountMicros string                     `json:"maxAverageCpmBidAmountMicros,omitempty"`
	RaiseBidForDeals             bool                       `json:"raiseBidForDeals,omitempty"`
}

type BiddingStrategy struct {
	FixedBid            *FixedBidStrategy           `json:"fixedBid,omitempty"`
	PerformanceGoalAuto *PerformanceGoalBidStrategy `json:"performanceGoalAutoBid,omitempty"`
	MaximizeSpendAuto   *MaximizeSpendBidStrategy   `json:"maximizeSpendAutoBid,omitempty"`
}

type InsertionOrder struct {
	Name                  string                `json:"name"`
	AdvertiserID          string                `json:"advertiserId"`
	CampaignID            string                `json:"campaignId"`
	InsertionOrderID      string                `json:"insertionOrderId"`
	DisplayName           string                `json:"displayName"`
	EntityStatus          EntityStatus          `json:"entityStatus"`
	InsertionOrderType    InsertionOrderType    `json:"insertionOrderType,omitempty"`
	Pacing                Pacing                `json:"pacing"`
	FrequencyCap          FrequencyCap          `json:"frequencyCap"`
	Budget                InsertionOrderBudget  `json:"budget"`
	KPI                   KPI                   `json:"kpi"`
	OptimizationObjective OptimizationObjective `json:"optimizationObjective"`
	BidStrategy           *BiddingStrategy      `json:"bidStrategy,omitempty"`
	UpdateTime            string                `json:"updateTime,omitempty"`
}

type LineItemFlight struct {
	Type      LineItemFlightDateType `json:"flightDateType"`
	DateRange *DateRange             `json:"dateRange,omitempty"`
}

type LineItemBudget struct {
	AllocationType LineItemBudgetAllocationType `json:"budgetAllocationType"`
	BudgetUnit     BudgetUnit                   `json:"budgetUnit,omitempty"`
	MaxAmount      string                       `json:"maxAmount,omitempty"`
}

type PartnerRevenueModel struct {
	MarkupType   PartnerRevenueMarkupType `json:"markupType"`
	MarkupAmount string                   `json:"markupAmount"`
}

type LineItem struct {
	Name                   string                       `json:"name"`
	AdvertiserID           string                       `json:"advertiserId"`
	CampaignID             string                       `json:"campaignId"`
	InsertionOrderID       string                       `json:"insertionOrderId"`
	LineItemID             string                       `json:"lineItemId"`
	DisplayName            string                       `json:"displayName"`
	LineItemType           LineItemType                 `json:"lineItemType"`
	EntityStatus           EntityStatus                 `json:"entityStatus"`
	Flight                 LineItemFlight               `json:"flight"`
	Budget                 LineItemBudget               `json:"budget"`
	Pacing                 Pacing                       `json:"pacing"`
	PartnerRevenueModel    PartnerRevenueModel          `json:"partnerRevenueModel"`
	BidStrategy            BiddingStrategy              `json:"bidStrategy"`
	FrequencyCap           FrequencyCap                 `json:"frequencyCap"`
	ContainsEUPoliticalAds EUPoliticalAdvertisingStatus `json:"containsEuPoliticalAds"`
	CreativeIDs            []string                     `json:"creativeIds,omitempty"`
	WarningMessages        []string                     `json:"warningMessages,omitempty"`
	UpdateTime             string                       `json:"updateTime,omitempty"`
}

type ListRequest struct {
	PageSize  int
	PageToken string
	Filter    string
	OrderBy   string
}

type Page[T any] struct {
	Items         []T
	NextPageToken string
}

type CreateCampaignRequest struct {
	DisplayName     string
	CampaignGoal    CampaignGoal
	CampaignFlight  CampaignFlight
	FrequencyCap    FrequencyCap
	CampaignBudgets []CampaignBudget
}

type UpdateCampaignRequest struct {
	DisplayName     *string
	EntityStatus    *EntityStatus
	CampaignGoal    *CampaignGoal
	CampaignFlight  *CampaignFlight
	FrequencyCap    *FrequencyCap
	CampaignBudgets *[]CampaignBudget
}

type CreateInsertionOrderRequest struct {
	CampaignID            string
	DisplayName           string
	InsertionOrderType    InsertionOrderType
	Pacing                Pacing
	FrequencyCap          FrequencyCap
	Budget                InsertionOrderBudget
	KPI                   KPI
	OptimizationObjective OptimizationObjective
	BidStrategy           *BiddingStrategy
}

type UpdateInsertionOrderRequest struct {
	DisplayName           *string
	EntityStatus          *EntityStatus
	Pacing                *Pacing
	FrequencyCap          *FrequencyCap
	Budget                *InsertionOrderBudget
	KPI                   *KPI
	OptimizationObjective *OptimizationObjective
	BidStrategy           *BiddingStrategy
}

type CreateLineItemRequest struct {
	InsertionOrderID       string
	DisplayName            string
	LineItemType           LineItemType
	Flight                 LineItemFlight
	Budget                 LineItemBudget
	Pacing                 Pacing
	PartnerRevenueModel    PartnerRevenueModel
	BidStrategy            BiddingStrategy
	FrequencyCap           FrequencyCap
	ContainsEUPoliticalAds EUPoliticalAdvertisingStatus
}

type UpdateLineItemRequest struct {
	DisplayName            *string
	EntityStatus           *EntityStatus
	Flight                 *LineItemFlight
	Budget                 *LineItemBudget
	Pacing                 *Pacing
	PartnerRevenueModel    *PartnerRevenueModel
	BidStrategy            *BiddingStrategy
	FrequencyCap           *FrequencyCap
	ContainsEUPoliticalAds *EUPoliticalAdvertisingStatus
}

type DuplicateLineItemRequest struct {
	TargetDisplayName      string
	ContainsEUPoliticalAds EUPoliticalAdvertisingStatus
}

type AdvertiserWorkflow interface {
	GetAdvertiser(context.Context, ...socialhub.CallOption) (Advertiser, error)
	ListAdvertisers(context.Context, ListRequest, ...socialhub.CallOption) (Page[Advertiser], error)
}

type CampaignWorkflow interface {
	GetCampaign(context.Context, string, ...socialhub.CallOption) (Campaign, error)
	ListCampaigns(context.Context, ListRequest, ...socialhub.CallOption) (Page[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
}

type InsertionOrderWorkflow interface {
	GetInsertionOrder(context.Context, string, ...socialhub.CallOption) (InsertionOrder, error)
	ListInsertionOrders(context.Context, ListRequest, ...socialhub.CallOption) (Page[InsertionOrder], error)
	CreateInsertionOrder(context.Context, CreateInsertionOrderRequest, ...socialhub.CallOption) (InsertionOrder, error)
	UpdateInsertionOrder(context.Context, string, UpdateInsertionOrderRequest, ...socialhub.CallOption) (InsertionOrder, error)
}

type LineItemWorkflow interface {
	GetLineItem(context.Context, string, ...socialhub.CallOption) (LineItem, error)
	ListLineItems(context.Context, ListRequest, ...socialhub.CallOption) (Page[LineItem], error)
	CreateLineItem(context.Context, CreateLineItemRequest, ...socialhub.CallOption) (LineItem, error)
	UpdateLineItem(context.Context, string, UpdateLineItemRequest, ...socialhub.CallOption) (LineItem, error)
	DuplicateLineItem(context.Context, string, DuplicateLineItemRequest, ...socialhub.CallOption) (LineItem, error)
}

type listAdvertisersResponse struct {
	Advertisers   []Advertiser `json:"advertisers"`
	NextPageToken string       `json:"nextPageToken"`
}

type listCampaignsResponse struct {
	Campaigns     []Campaign `json:"campaigns"`
	NextPageToken string     `json:"nextPageToken"`
}

type listInsertionOrdersResponse struct {
	InsertionOrders []InsertionOrder `json:"insertionOrders"`
	NextPageToken   string           `json:"nextPageToken"`
}

type listLineItemsResponse struct {
	LineItems     []LineItem `json:"lineItems"`
	NextPageToken string     `json:"nextPageToken"`
}
