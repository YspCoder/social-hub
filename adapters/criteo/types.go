package criteo

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type CampaignGoal string

const (
	GoalAcquisition CampaignGoal = "acquisition"
	GoalRetention   CampaignGoal = "retention"
)

type SpendLimitType string

const (
	SpendLimitCapped   SpendLimitType = "capped"
	SpendLimitUncapped SpendLimitType = "uncapped"
)

type SpendLimitRenewal string

const (
	RenewalUndefined SpendLimitRenewal = "undefined"
	RenewalDaily     SpendLimitRenewal = "daily"
	RenewalMonthly   SpendLimitRenewal = "monthly"
	RenewalLifetime  SpendLimitRenewal = "lifetime"
)

type BudgetAutomationObjective string

const (
	AutomationConversions BudgetAutomationObjective = "conversions"
	AutomationRevenue     BudgetAutomationObjective = "revenue"
	AutomationVisits      BudgetAutomationObjective = "visits"
	AutomationVideoViews  BudgetAutomationObjective = "videoViews"
)

type MediaType string

const (
	MediaDisplay MediaType = "display"
	MediaVideo   MediaType = "video"
)

type Objective string

const (
	ObjectiveCustomAction     Objective = "customAction"
	ObjectiveClicks           Objective = "clicks"
	ObjectiveConversions      Objective = "conversions"
	ObjectiveDisplays         Objective = "displays"
	ObjectiveAppPromotion     Objective = "appPromotion"
	ObjectiveRevenue          Objective = "revenue"
	ObjectiveStoreConversions Objective = "storeConversions"
	ObjectiveValue            Objective = "value"
	ObjectiveReach            Objective = "reach"
	ObjectiveVisits           Objective = "visits"
	ObjectiveVideoViews       Objective = "videoViews"
)

type CostController string

const (
	CostCOS         CostController = "COS"
	CostMaxCPC      CostController = "maxCPC"
	CostCPI         CostController = "CPI"
	CostCPM         CostController = "CPM"
	CostCPO         CostController = "CPO"
	CostCPSV        CostController = "CPSV"
	CostCPV         CostController = "CPV"
	CostDailyBudget CostController = "dailyBudget"
	CostTargetCPM   CostController = "targetCPM"
)

type BudgetStrategy string

const (
	BudgetCapped   BudgetStrategy = "capped"
	BudgetUncapped BudgetStrategy = "uncapped"
)

type BudgetRenewal string

const (
	BudgetUndefined BudgetRenewal = "undefined"
	BudgetDaily     BudgetRenewal = "daily"
	BudgetMonthly   BudgetRenewal = "monthly"
	BudgetLifetime  BudgetRenewal = "lifetime"
	BudgetWeekly    BudgetRenewal = "weekly"
)

type DeliverySmoothing string

const (
	DeliveryAccelerated DeliverySmoothing = "accelerated"
	DeliveryStandard    DeliverySmoothing = "standard"
)

type DeliveryWeek string

const (
	WeekUndefined           DeliveryWeek = "undefined"
	WeekMondayToSunday      DeliveryWeek = "mondayToSunday"
	WeekTuesdayToMonday     DeliveryWeek = "tuesdayToMonday"
	WeekWednesdayToTuesday  DeliveryWeek = "wednesdayToTuesday"
	WeekThursdayToWednesday DeliveryWeek = "thursdayToWednesday"
	WeekFridayToThursday    DeliveryWeek = "fridayToThursday"
	WeekSaturdayToFriday    DeliveryWeek = "saturdayToFriday"
	WeekSundayToSaturday    DeliveryWeek = "sundayToSaturday"
)

type AttributionMethod string

const (
	AttributionUnknown                   AttributionMethod = "unknown"
	AttributionCriteo                    AttributionMethod = "criteoAttribution"
	AttributionGoogleAnalyticsLastClick  AttributionMethod = "googleAnalyticsLastClick"
	AttributionGoogleAnalyticsDataDriven AttributionMethod = "googleAnalyticsDataDriven"
	AttributionLastClick                 AttributionMethod = "lastClick"
	AttributionPostClick                 AttributionMethod = "postClick"
)

type LookbackWindow string

const (
	LookbackUnknown LookbackWindow = "unknown"
	Lookback30M     LookbackWindow = "30M"
	Lookback24H     LookbackWindow = "24H"
	Lookback7D      LookbackWindow = "7D"
	Lookback30D     LookbackWindow = "30D"
)

type ActivationStatus string

const (
	ActivationOn  ActivationStatus = "on"
	ActivationOff ActivationStatus = "off"
)

type DeliveryStatus string

const (
	DeliveryDraft         DeliveryStatus = "draft"
	DeliveryInactive      DeliveryStatus = "inactive"
	DeliveryLive          DeliveryStatus = "live"
	DeliveryNotLive       DeliveryStatus = "notLive"
	DeliveryPausing       DeliveryStatus = "pausing"
	DeliveryPaused        DeliveryStatus = "paused"
	DeliveryScheduled     DeliveryStatus = "scheduled"
	DeliveryEnded         DeliveryStatus = "ended"
	DeliveryNotDelivering DeliveryStatus = "notDelivering"
	DeliveryArchived      DeliveryStatus = "archived"
)

type Frequency string

const (
	FrequencyHourly   Frequency = "hourly"
	FrequencyDaily    Frequency = "daily"
	FrequencyLifetime Frequency = "lifetime"
	FrequencyAdvanced Frequency = "advanced"
)

type TargetingOperand string

const (
	OperandIn    TargetingOperand = "in"
	OperandNotIn TargetingOperand = "notIn"
)

type Device string

const (
	DeviceOther   Device = "other"
	DeviceDesktop Device = "desktop"
	DeviceMobile  Device = "mobile"
	DeviceTablet  Device = "tablet"
)

type Environment string

const (
	EnvironmentWeb   Environment = "web"
	EnvironmentInApp Environment = "inApp"
)

type OperatingSystem string

const (
	OSAndroid OperatingSystem = "android"
	OSIOS     OperatingSystem = "ios"
	OSUnknown OperatingSystem = "unknown"
)

// Advertiser is one advertiser visible in the authenticated portfolio.
type Advertiser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NullableFloat represents Criteo's {"value": number|null} patch/read shape.
type NullableFloat struct {
	Value *float64 `json:"value"`
}

// NullableTime represents Criteo's {"value": date-time|null} patch/read shape.
type NullableTime struct {
	Value *string `json:"value"`
}

type CampaignBudgetAutomation struct {
	Enabled                    bool                      `json:"enabled"`
	AdSetOptimizationObjective BudgetAutomationObjective `json:"adSetOptimizationObjective,omitempty"`
}

type CampaignSpendLimit struct {
	Type    SpendLimitType    `json:"spendLimitType"`
	Amount  NullableFloat     `json:"spendLimitAmount"`
	Renewal SpendLimitRenewal `json:"spendLimitRenewal"`
}

type ScheduledSpendLimit struct {
	ID        string            `json:"id"`
	Type      SpendLimitType    `json:"spendLimitType"`
	Amount    NullableFloat     `json:"spendLimitAmount"`
	Renewal   SpendLimitRenewal `json:"spendLimitRenewal"`
	StartDate string            `json:"startDate"`
}

// Campaign is a Criteo marketing campaign.
type Campaign struct {
	ID                   string                    `json:"id"`
	AdvertiserID         string                    `json:"advertiserId"`
	Name                 string                    `json:"name"`
	Goal                 CampaignGoal              `json:"goal"`
	SpendLimit           CampaignSpendLimit        `json:"spendLimit"`
	ScheduledSpendLimits []ScheduledSpendLimit     `json:"scheduledSpendLimits,omitempty"`
	BudgetAutomation     *CampaignBudgetAutomation `json:"budgetAutomation,omitempty"`
}

type CreateCampaignSpendLimit struct {
	Type    SpendLimitType
	Amount  *float64
	Renewal SpendLimitRenewal
}

type CreateBudgetAutomation struct {
	Enabled   bool
	Objective BudgetAutomationObjective
}

type CreateCampaignRequest struct {
	Name             string
	Goal             CampaignGoal
	SpendLimit       CreateCampaignSpendLimit
	BudgetAutomation *CreateBudgetAutomation
}

type CampaignSearchRequest struct {
	CampaignIDs []string
}

type PatchCampaignSpendLimit struct {
	Type    SpendLimitType    `json:"spendLimitType,omitempty"`
	Amount  *NullableFloat    `json:"spendLimitAmount,omitempty"`
	Renewal SpendLimitRenewal `json:"spendLimitRenewal,omitempty"`
}

type PatchBudgetAutomation struct {
	Enabled   *bool                     `json:"enabled,omitempty"`
	Objective BudgetAutomationObjective `json:"-"`
}

type ScheduledSpendLimitCreation struct {
	Type      SpendLimitType    `json:"spendLimitType"`
	Amount    *NullableFloat    `json:"spendLimitAmount,omitempty"`
	Renewal   SpendLimitRenewal `json:"spendLimitRenewal,omitempty"`
	StartDate string            `json:"startDate"`
}

type ScheduledSpendLimitUpdate struct {
	ID        string            `json:"id"`
	Type      SpendLimitType    `json:"spendLimitType"`
	Amount    *NullableFloat    `json:"spendLimitAmount,omitempty"`
	Renewal   SpendLimitRenewal `json:"spendLimitRenewal,omitempty"`
	StartDate string            `json:"startDate"`
}

type PatchScheduledSpendLimits struct {
	Creations []ScheduledSpendLimitCreation `json:"scheduledSpendLimitCreations,omitempty"`
	Updates   []ScheduledSpendLimitUpdate   `json:"scheduledSpendLimitUpdates,omitempty"`
	Deletions []string                      `json:"-"`
}

type UpdateCampaignRequest struct {
	SpendLimit          *PatchCampaignSpendLimit
	BudgetAutomation    *PatchBudgetAutomation
	ScheduledSpendLimit *PatchScheduledSpendLimits
}

type AttributionConfiguration struct {
	Method         AttributionMethod `json:"attributionMethod,omitempty"`
	LookbackWindow LookbackWindow    `json:"lookbackWindow,omitempty"`
}

type CreateAdSetBidding struct {
	CostController CostController `json:"costController"`
	BidAmount      *float64       `json:"bidAmount,omitempty"`
}

type CreateAdSetBudget struct {
	Strategy          BudgetStrategy    `json:"budgetStrategy"`
	Amount            *float64          `json:"budgetAmount,omitempty"`
	Renewal           BudgetRenewal     `json:"budgetRenewal,omitempty"`
	DeliverySmoothing DeliverySmoothing `json:"budgetDeliverySmoothing,omitempty"`
	DeliveryWeek      DeliveryWeek      `json:"budgetDeliveryWeek,omitempty"`
}

type CreateAdSetSchedule struct {
	StartDate string  `json:"startDate"`
	EndDate   *string `json:"endDate,omitempty"`
}

type DeliveryLimitations struct {
	Devices          []Device          `json:"devices,omitempty"`
	Environments     []Environment     `json:"environments,omitempty"`
	OperatingSystems []OperatingSystem `json:"operatingSystems,omitempty"`
}

type FrequencyCapping struct {
	Frequency          Frequency `json:"frequency,omitempty"`
	MaximumImpressions int       `json:"maximumImpressions,omitempty"`
}

type TargetingRule struct {
	Operand TargetingOperand `json:"operand"`
	Values  []string         `json:"values"`
}

type CreateGeoLocation struct {
	Countries    *TargetingRule `json:"countries,omitempty"`
	Subdivisions *TargetingRule `json:"subdivisions,omitempty"`
	ZipCodes     *TargetingRule `json:"zipCodes,omitempty"`
}

type CreateAdSetTargeting struct {
	DeliveryLimitations *DeliveryLimitations `json:"deliveryLimitations,omitempty"`
	FrequencyCapping    FrequencyCapping     `json:"frequencyCapping"`
	GeoLocation         *CreateGeoLocation   `json:"geoLocation,omitempty"`
}

type CreateAdSetRequest struct {
	Name                     string
	DatasetID                string
	Objective                Objective
	MediaType                MediaType
	Schedule                 CreateAdSetSchedule
	Bidding                  CreateAdSetBidding
	Budget                   CreateAdSetBudget
	Targeting                CreateAdSetTargeting
	TrackingCode             string
	AttributionConfiguration *AttributionConfiguration
}

type AdSetBidding struct {
	CostController CostController `json:"costController"`
	BidAmount      *float64       `json:"bidAmount,omitempty"`
}

type AdSetBudget struct {
	Strategy          BudgetStrategy    `json:"budgetStrategy"`
	Amount            *float64          `json:"budgetAmount,omitempty"`
	Renewal           BudgetRenewal     `json:"budgetRenewal"`
	DeliverySmoothing DeliverySmoothing `json:"budgetDeliverySmoothing"`
	DeliveryWeek      DeliveryWeek      `json:"budgetDeliveryWeek"`
}

type AdSetSchedule struct {
	ActivationStatus ActivationStatus `json:"activationStatus"`
	DeliveryStatus   DeliveryStatus   `json:"deliveryStatus"`
	StartDate        NullableTime     `json:"startDate"`
	EndDate          NullableTime     `json:"endDate"`
}

type NullableTargetingRule struct {
	Value *TargetingRule `json:"value"`
}

type AdSetGeoLocation struct {
	Countries    NullableTargetingRule `json:"countries"`
	Subdivisions NullableTargetingRule `json:"subdivisions"`
	ZipCodes     NullableTargetingRule `json:"zipCodes"`
}

type AdSetTargeting struct {
	DeliveryLimitations *DeliveryLimitations `json:"deliveryLimitations,omitempty"`
	FrequencyCapping    json.RawMessage      `json:"frequencyCapping,omitempty"`
	GeoLocation         *AdSetGeoLocation    `json:"geoLocation,omitempty"`
}

// AdSet is a Criteo Ad Set. New Ad Sets must be returned off and draft.
type AdSet struct {
	ID                       string                    `json:"id"`
	AdvertiserID             string                    `json:"advertiserId"`
	CampaignID               string                    `json:"campaignId"`
	DatasetID                string                    `json:"datasetId"`
	Name                     string                    `json:"name"`
	Objective                Objective                 `json:"objective"`
	MediaType                MediaType                 `json:"mediaType"`
	DestinationEnvironment   string                    `json:"destinationEnvironment,omitempty"`
	VideoChannel             string                    `json:"videoChannel,omitempty"`
	AttributionConfiguration *AttributionConfiguration `json:"attributionConfiguration,omitempty"`
	Bidding                  *AdSetBidding             `json:"bidding,omitempty"`
	Budget                   *AdSetBudget              `json:"budget,omitempty"`
	Schedule                 AdSetSchedule             `json:"schedule"`
	Targeting                *AdSetTargeting           `json:"targeting,omitempty"`
}

type AdSetSearchRequest struct {
	AdSetIDs    []string
	CampaignIDs []string
}

type PatchAdSetBidding struct {
	BidAmount *NullableFloat `json:"bidAmount,omitempty"`
}

type PatchAdSetBudget struct {
	Amount            *NullableFloat     `json:"budgetAmount,omitempty"`
	Strategy          *BudgetStrategy    `json:"budgetStrategy,omitempty"`
	Renewal           *BudgetRenewal     `json:"budgetRenewal,omitempty"`
	DeliverySmoothing *DeliverySmoothing `json:"budgetDeliverySmoothing,omitempty"`
	DeliveryWeek      *DeliveryWeek      `json:"budgetDeliveryWeek,omitempty"`
}

type PatchAdSetSchedule struct {
	StartDate *NullableTime `json:"startDate,omitempty"`
	EndDate   *NullableTime `json:"endDate,omitempty"`
}

type UpdateAdSetRequest struct {
	Name                     *string
	AttributionConfiguration *AttributionConfiguration
	Bidding                  *PatchAdSetBidding
	Budget                   *PatchAdSetBudget
	Schedule                 *PatchAdSetSchedule
}

type AdSetReportStatus string

const (
	ReportAdSetActive     AdSetReportStatus = "Active"
	ReportAdSetNotRunning AdSetReportStatus = "NotRunning"
	ReportAdSetDead       AdSetReportStatus = "Dead"
)

type Dimension string

const (
	DimensionAdSetID      Dimension = "AdsetId"
	DimensionAdSet        Dimension = "Adset"
	DimensionAdvertiserID Dimension = "AdvertiserId"
	DimensionAdvertiser   Dimension = "Advertiser"
	DimensionDay          Dimension = "Day"
	DimensionCampaignID   Dimension = "CampaignId"
	DimensionCampaign     Dimension = "Campaign"
	DimensionAdID         Dimension = "AdId"
	DimensionAd           Dimension = "Ad"
	DimensionDevice       Dimension = "Device"
)

type Metric string

const (
	MetricClicks         Metric = "Clicks"
	MetricDisplays       Metric = "Displays"
	MetricAdvertiserCost Metric = "AdvertiserCost"
)

type StatisticsReportRequest struct {
	Currency    string
	Dimensions  []Dimension
	Metrics     []Metric
	StartDate   string
	EndDate     string
	Timezone    string
	AdSetIDs    []string
	AdSetNames  []string
	AdSetStatus []AdSetReportStatus
}

// StatisticsReport preserves the dynamic JSON columns requested by the caller.
type StatisticsReport struct {
	ContentType string
	Data        json.RawMessage
}

type AdvertiserWorkflow interface {
	ListAdvertisers(context.Context, ...socialhub.CallOption) ([]Advertiser, error)
	ValidateConfiguredAdvertiser(context.Context, ...socialhub.CallOption) (Advertiser, error)
}

type CampaignWorkflow interface {
	GetCampaign(context.Context, string, ...socialhub.CallOption) (Campaign, error)
	SearchCampaigns(context.Context, CampaignSearchRequest, ...socialhub.CallOption) ([]Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
}

type AdSetWorkflow interface {
	GetAdSet(context.Context, string, ...socialhub.CallOption) (AdSet, error)
	SearchAdSets(context.Context, AdSetSearchRequest, ...socialhub.CallOption) ([]AdSet, error)
	CreateAdSet(context.Context, string, CreateAdSetRequest, ...socialhub.CallOption) (AdSet, error)
	UpdateAdSet(context.Context, string, UpdateAdSetRequest, ...socialhub.CallOption) (AdSet, error)
	StartAdSet(context.Context, string, ...socialhub.CallOption) (AdSet, error)
	StopAdSet(context.Context, string, ...socialhub.CallOption) (AdSet, error)
}

type StatisticsWorkflow interface {
	Report(context.Context, StatisticsReportRequest, ...socialhub.CallOption) (StatisticsReport, error)
}

type entityResource[T any] struct {
	Type       string `json:"type"`
	ID         string `json:"id,omitempty"`
	Attributes T      `json:"attributes"`
}

type advertiserAttributes struct {
	AdvertiserName string `json:"advertiserName"`
}

type campaignAttributes struct {
	AdvertiserID         string                  `json:"advertiserId"`
	ID                   string                  `json:"id"`
	Name                 string                  `json:"name"`
	Goal                 CampaignGoal            `json:"goal"`
	SpendLimit           CampaignSpendLimit      `json:"spendLimit"`
	ScheduledSpendLimits []ScheduledSpendLimit   `json:"scheduledSpendLimits"`
	BudgetAutomation     *campaignAutomationWire `json:"budgetAutomation"`
}

type campaignAutomationWire struct {
	Enabled                      bool `json:"enabled"`
	AutomatedBudgetConfiguration struct {
		AdSetOptimizationObjective BudgetAutomationObjective `json:"adSetOptimizationObjective"`
	} `json:"automatedBudgetConfiguration"`
}

type campaignWriteEnvelope struct {
	Data entityResource[createCampaignAttributes] `json:"data"`
}

type createCampaignAttributes struct {
	AdvertiserID     string                       `json:"advertiserId"`
	Name             string                       `json:"name"`
	Goal             string                       `json:"goal"`
	SpendLimit       createCampaignSpendLimitWire `json:"spendLimit"`
	BudgetAutomation *budgetAutomationWrite       `json:"budgetAutomation,omitempty"`
}

type createCampaignSpendLimitWire struct {
	Type    SpendLimitType    `json:"spendLimitType"`
	Amount  *float64          `json:"spendLimitAmount,omitempty"`
	Renewal SpendLimitRenewal `json:"spendLimitRenewal,omitempty"`
}

type budgetAutomationWrite struct {
	Enabled             bool `json:"enabled"`
	BudgetConfiguration struct {
		AdSetObjectives BudgetAutomationObjective `json:"adSetObjectives"`
	} `json:"budgetConfiguration"`
}

type campaignSearchEnvelope struct {
	Filters struct {
		AdvertiserIDs []string `json:"advertiserIds"`
		CampaignIDs   []string `json:"campaignIds,omitempty"`
	} `json:"filters"`
}

type campaignPatchEnvelope struct {
	Data []entityResource[campaignPatchAttributes] `json:"data"`
}

type campaignPatchAttributes struct {
	SpendLimit          *PatchCampaignSpendLimit `json:"spendLimit,omitempty"`
	BudgetAutomation    *budgetAutomationPatch   `json:"budgetAutomation,omitempty"`
	ScheduledSpendLimit *scheduledSpendPatchWire `json:"scheduledSpendLimit,omitempty"`
}

type budgetAutomationPatch struct {
	Enabled             *bool `json:"enabled,omitempty"`
	BudgetConfiguration *struct {
		AdSetObjectives BudgetAutomationObjective `json:"adSetObjectives"`
	} `json:"budgetConfiguration,omitempty"`
}

type scheduledSpendPatchWire struct {
	Creations []ScheduledSpendLimitCreation `json:"scheduledSpendLimitCreations,omitempty"`
	Updates   []ScheduledSpendLimitUpdate   `json:"scheduledSpendLimitUpdates,omitempty"`
	Deletions []struct {
		ID string `json:"id"`
	} `json:"scheduledSpendLimitDeletions,omitempty"`
}

type createAdSetAttributes struct {
	CampaignID               string                    `json:"campaignId"`
	Name                     string                    `json:"name"`
	DatasetID                string                    `json:"datasetId"`
	Objective                Objective                 `json:"objective"`
	MediaType                MediaType                 `json:"mediaType"`
	Schedule                 CreateAdSetSchedule       `json:"schedule"`
	Bidding                  CreateAdSetBidding        `json:"bidding"`
	Budget                   CreateAdSetBudget         `json:"budget"`
	Targeting                CreateAdSetTargeting      `json:"targeting"`
	TrackingCode             string                    `json:"trackingCode"`
	AttributionConfiguration *AttributionConfiguration `json:"attributionConfiguration,omitempty"`
}

type adSetSearchEnvelope struct {
	Filters struct {
		AdvertiserIDs []string `json:"advertiserIds"`
		AdSetIDs      []string `json:"adSetIds,omitempty"`
		CampaignIDs   []string `json:"campaignIds,omitempty"`
	} `json:"filters"`
}

type adSetPatchAttributes struct {
	Name                     *string                   `json:"name,omitempty"`
	AttributionConfiguration *AttributionConfiguration `json:"attributionConfiguration,omitempty"`
	Bidding                  *PatchAdSetBidding        `json:"bidding,omitempty"`
	Budget                   *PatchAdSetBudget         `json:"budget,omitempty"`
	Scheduling               *PatchAdSetSchedule       `json:"scheduling,omitempty"`
}

type adSetPatchEnvelope struct {
	Data []entityResource[adSetPatchAttributes] `json:"data"`
}

type adSetIDEnvelope struct {
	Data []idResource `json:"data"`
}

type idResource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type statisticsReportWire struct {
	AdvertiserIDs string              `json:"advertiserIds"`
	Currency      string              `json:"currency"`
	Dimensions    []Dimension         `json:"dimensions"`
	Metrics       []Metric            `json:"metrics"`
	StartDate     string              `json:"startDate"`
	EndDate       string              `json:"endDate"`
	Format        string              `json:"format"`
	Timezone      string              `json:"timezone,omitempty"`
	AdSetIDs      []string            `json:"adSetIds,omitempty"`
	AdSetNames    []string            `json:"adSetNames,omitempty"`
	AdSetStatus   []AdSetReportStatus `json:"adSetStatus,omitempty"`
}
