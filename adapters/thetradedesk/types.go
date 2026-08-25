package thetradedesk

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type Availability string

const (
	AvailabilityAvailable Availability = "Available"
	AvailabilityArchived  Availability = "Archived"
)

type CampaignVersion string

const CampaignVersionKokai CampaignVersion = "Kokai"

type CampaignType string

const (
	CampaignTypeStandard               CampaignType = "Standard"
	CampaignTypeProgrammaticGuaranteed CampaignType = "ProgrammaticGuaranteed"
)

type CampaignBudgetingVersion string

const (
	CampaignBudgetingSolimar CampaignBudgetingVersion = "Solimar"
	CampaignBudgetingKokai   CampaignBudgetingVersion = "Kokai"
)

type PacingMode string

const (
	PacingOff              PacingMode = "Off"
	PacingToEndOfFlight    PacingMode = "PaceToEndOfFlight"
	PacingAhead            PacingMode = "PaceAhead"
	PacingAsSoonAsPossible PacingMode = "PaceAsSoonAsPossible"
)

type Channel string

const (
	ChannelDisplay          Channel = "Display"
	ChannelVideo            Channel = "Video"
	ChannelAudio            Channel = "Audio"
	ChannelTV               Channel = "TV"
	ChannelNativeDisplay    Channel = "NativeDisplay"
	ChannelNativeVideo      Channel = "NativeVideo"
	ChannelDigitalOutOfHome Channel = "DigitalOutOfHome"
)

type NewBuyerTargetType string

const (
	NewBuyerTargetFeatured NewBuyerTargetType = "Featured"
	NewBuyerTargetTotal    NewBuyerTargetType = "Total"
)

type CampaignSortField string

const (
	SortCampaignName                     CampaignSortField = "Name"
	SortCampaignDescription              CampaignSortField = "Description"
	SortCampaignBudget                   CampaignSortField = "Budget"
	SortCampaignBudgetInImpressions      CampaignSortField = "BudgetInImpressions"
	SortCampaignDailyBudget              CampaignSortField = "DailyBudget"
	SortCampaignDailyBudgetInImpressions CampaignSortField = "DailyBudgetInImpressions"
	SortCampaignCreatedAtUTC             CampaignSortField = "CreatedAtUtc"
	SortCampaignLastUpdatedAtUTC         CampaignSortField = "LastUpdatedAtUtc"
)

// Money preserves the provider's JSON decimal spelling while retaining the
// currency required by the Platform API contract.
type Money struct {
	Amount       json.Number `json:"Amount"`
	CurrencyCode string      `json:"CurrencyCode"`
}

// CampaignGoal is the typed REST PrimaryGoal union. Exactly one field must be
// set when creating a campaign.
type CampaignGoal struct {
	CPAInAdvertiserCurrency   *Money             `json:"CPAInAdvertiserCurrency,omitempty"`
	CPCInAdvertiserCurrency   *Money             `json:"CPCInAdvertiserCurrency,omitempty"`
	CPCVInAdvertiserCurrency  *Money             `json:"CPCVInAdvertiserCurrency,omitempty"`
	CTRInPercent              *float64           `json:"CTRInPercent,omitempty"`
	MaximizeLTVReach          bool               `json:"MaximizeLtvIncrementalReach,omitempty"`
	MaximizeReach             bool               `json:"MaximizeReach,omitempty"`
	MiaozhenOTPInPercent      *float64           `json:"MiaozhenOTPInPercent,omitempty"`
	NewBuyerTargetValue       NewBuyerTargetType `json:"NewBuyerTargetValue,omitempty"`
	NielsenOTPInPercent       *float64           `json:"NielsenOTPInPercent,omitempty"`
	ReturnOnAdSpendPercent    *float64           `json:"ReturnOnAdSpendPercent,omitempty"`
	VCPMInAdvertiserCurrency  *Money             `json:"VCPMInAdvertiserCurrency,omitempty"`
	VCRInPercent              *float64           `json:"VCRInPercent,omitempty"`
	ViewabilityInPercent      *float64           `json:"ViewabilityInPercent,omitempty"`
	MaximizeConversionRevenue bool               `json:"MaximizeConversionRevenue,omitempty"`
}

type CustomROASConfig struct {
	ClickWeight       *float64 `json:"CustomROASClickWeight,omitempty"`
	ViewthroughWeight *float64 `json:"CustomROASViewthroughWeight,omitempty"`
	Weight            *float64 `json:"CustomROASWeight,omitempty"`
	Include           bool     `json:"IncludeInCustomROAS,omitempty"`
}

type ConversionReportingColumnInput struct {
	TrackingTagID                 string            `json:"TrackingTagId"`
	CrossDeviceAttributionModelID *string           `json:"CrossDeviceAttributionModelId,omitempty"`
	CustomROAS                    *CustomROASConfig `json:"CustomROASConfig,omitempty"`
	IncludeInCustomCPA            bool              `json:"IncludeInCustomCPA,omitempty"`
	ReportingColumnID             int32             `json:"ReportingColumnId,omitempty"`
	Weight                        *float64          `json:"Weight,omitempty"`
}

type ConversionReportingColumn struct {
	ConversionReportingColumnInput
	TrackingTagName *string         `json:"TrackingTagName,omitempty"`
	ProductListInfo json.RawMessage `json:"ProductListInfo,omitempty"`
}

type ResponseMeta struct {
	RequestID string
}

// Advertiser contains the stable advertiser fields used by this adapter.
type Advertiser struct {
	ID                   string          `json:"AdvertiserId"`
	Name                 string          `json:"AdvertiserName"`
	PartnerID            string          `json:"PartnerId"`
	Availability         Availability    `json:"Availability"`
	Country              string          `json:"Country"`
	CurrencyCode         string          `json:"CurrencyCode"`
	Description          *string         `json:"Description"`
	DomainAddress        string          `json:"DomainAddress"`
	IndustryCategoryID   int64           `json:"IndustryCategoryId"`
	AdvertiserCategory   json.RawMessage `json:"AdvertiserCategory"`
	VettingStatus        json.RawMessage `json:"VettingStatus"`
	ClickLookbackSeconds int32           `json:"AttributionClickLookbackWindowInSeconds"`
	ViewLookbackSeconds  int32           `json:"AttributionImpressionLookbackWindowInSeconds"`
	Meta                 ResponseMeta    `json:"-"`
}

// Campaign contains the stable fields shared by REST Campaign reads and writes.
type Campaign struct {
	ID                         string                      `json:"CampaignId"`
	AdvertiserID               string                      `json:"AdvertiserId"`
	Name                       string                      `json:"CampaignName"`
	Description                string                      `json:"Description"`
	Availability               Availability                `json:"Availability"`
	Budget                     *Money                      `json:"Budget"`
	BudgetInImpressions        *int64                      `json:"BudgetInImpressions"`
	DailyBudget                *Money                      `json:"DailyBudget"`
	DailyBudgetInImpressions   *int64                      `json:"DailyBudgetInImpressions"`
	StartDate                  string                      `json:"StartDate"`
	EndDate                    *string                     `json:"EndDate"`
	TimeZone                   string                      `json:"TimeZone"`
	PacingMode                 PacingMode                  `json:"PacingMode"`
	Type                       CampaignType                `json:"CampaignType"`
	Version                    CampaignVersion             `json:"Version"`
	BudgetingVersion           CampaignBudgetingVersion    `json:"BudgetingVersion"`
	PrimaryChannel             Channel                     `json:"PrimaryChannel"`
	Objective                  string                      `json:"Objective"`
	PrimaryGoal                json.RawMessage             `json:"PrimaryGoal"`
	SecondaryGoal              json.RawMessage             `json:"SecondaryGoal"`
	TertiaryGoal               json.RawMessage             `json:"TertiaryGoal"`
	SeedID                     string                      `json:"SeedId"`
	PurchaseOrderNumber        *string                     `json:"PurchaseOrderNumber"`
	ConversionReportingColumns []ConversionReportingColumn `json:"CampaignConversionReportingColumns"`
	AutoAllocatorEnabled       bool                        `json:"AutoAllocatorEnabled"`
	AutoPrioritizationEnabled  bool                        `json:"AutoPrioritizationEnabled"`
	CreatedAtUTC               *string                     `json:"CreatedAtUTC"`
	LastUpdatedAtUTC           *string                     `json:"LastUpdatedAtUTC"`
	Meta                       ResponseMeta                `json:"-"`
}

type CampaignSort struct {
	Field     CampaignSortField `json:"FieldId"`
	Ascending bool              `json:"Ascending"`
}

type CampaignQuery struct {
	PageStartIndex int32
	// Nil uses social-hub's bounded default of 100. Pointing to zero requests counts only.
	PageSize       *int32
	Availabilities []Availability
	SearchTerms    []string
	SortFields     []CampaignSort
}

type CampaignPage struct {
	Campaigns            []Campaign
	ResultCount          *int64
	TotalFilteredCount   *int64
	TotalUnfilteredCount *int64
	Meta                 ResponseMeta
}

// CreateCampaignRequest supports the REST single-flight creation path. Use
// the dedicated Campaign Flight API for multi-flight campaigns.
type CreateCampaignRequest struct {
	Name                       string
	Description                string
	ConversionReportingColumns []ConversionReportingColumnInput
	PrimaryGoal                CampaignGoal
	Budget                     *Money
	BudgetInImpressions        *int64
	DailyBudget                *Money
	DailyBudgetInImpressions   *int64
	StartDate                  string
	EndDate                    *string
	TimeZone                   string
	PacingMode                 PacingMode
	Type                       CampaignType
	Version                    CampaignVersion
	BudgetingVersion           CampaignBudgetingVersion
	PrimaryChannel             Channel
	SeedID                     string
	PurchaseOrderNumber        *string
}

// UpdateCampaignRequest is a partial REST update. Clear flags intentionally
// distinguish an omitted property from an explicit JSON null.
type UpdateCampaignRequest struct {
	Name                          *string
	Description                   *string
	Availability                  *Availability
	Budget                        *Money
	BudgetInImpressions           *int64
	ClearBudgetInImpressions      bool
	DailyBudget                   *Money
	ClearDailyBudget              bool
	DailyBudgetInImpressions      *int64
	ClearDailyBudgetInImpressions bool
	StartDate                     *string
	EndDate                       *string
	ClearEndDate                  bool
	TimeZone                      *string
	PacingMode                    *PacingMode
	PrimaryChannel                *Channel
	SeedID                        *string
	PurchaseOrderNumber           *string
	ClearPurchaseOrderNumber      bool
	ConversionReportingColumns    *[]ConversionReportingColumnInput
}

type AdvertiserWorkflow interface {
	GetAdvertiser(context.Context, ...socialhub.CallOption) (*Advertiser, error)
}

type CampaignWorkflow interface {
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	QueryCampaigns(context.Context, CampaignQuery, ...socialhub.CallOption) (*CampaignPage, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
}

var (
	_ AdvertiserWorkflow = (*Client)(nil)
	_ CampaignWorkflow   = (*Client)(nil)
)
