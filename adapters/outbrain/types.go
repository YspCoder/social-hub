package outbrain

import (
	"context"
	"encoding/json"
	"strings"

	"social-hub/pkg/socialhub"
)

type BudgetType string

const (
	BudgetCampaign BudgetType = "CAMPAIGN"
	BudgetMonthly  BudgetType = "MONTHLY"
	BudgetDaily    BudgetType = "DAILY"
)

type PacingType string

const (
	PacingSpendASAP   PacingType = "SPEND_ASAP"
	PacingAutomatic   PacingType = "AUTOMATIC"
	PacingDailyTarget PacingType = "DAILY_TARGET"
)

type Marketer struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Enabled      bool            `json:"enabled"`
	CreationTime string          `json:"creationTime"`
	LastModified string          `json:"lastModified"`
	BlockedSites json.RawMessage `json:"blockedSites,omitempty"`
	Bids         json.RawMessage `json:"bids,omitempty"`
	Account      json.RawMessage `json:"account,omitempty"`
}

type Budget struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Shared          bool       `json:"shared"`
	Amount          float64    `json:"amount"`
	Currency        string     `json:"currency"`
	AmountRemaining float64    `json:"amountRemaining"`
	AmountSpent     float64    `json:"amountSpent"`
	CreationTime    string     `json:"creationTime"`
	LastModified    string     `json:"lastModified"`
	StartDate       string     `json:"startDate"`
	EndDate         string     `json:"endDate,omitempty"`
	RunForever      bool       `json:"runForever"`
	Type            BudgetType `json:"type"`
	Pacing          PacingType `json:"pacing"`
	DailyTarget     float64    `json:"dailyTarget,omitempty"`
	MaximumAmount   float64    `json:"maximumAmount,omitempty"`
}

type CreateBudgetRequest struct {
	Name        string     `json:"name"`
	Amount      float64    `json:"amount"`
	StartDate   string     `json:"startDate"`
	EndDate     string     `json:"endDate"`
	Pacing      PacingType `json:"pacing"`
	Type        BudgetType `json:"type"`
	DailyTarget *float64   `json:"dailyTarget,omitempty"`
}

type UpdateBudgetRequest struct {
	Name        *string     `json:"name,omitempty"`
	Amount      *float64    `json:"amount,omitempty"`
	EndDate     *string     `json:"endDate,omitempty"`
	Pacing      *PacingType `json:"pacing,omitempty"`
	DailyTarget *float64    `json:"dailyTarget,omitempty"`
}

type CampaignTargeting struct {
	Platform         []string `json:"platform,omitempty"`
	Locations        []string `json:"locations,omitempty"`
	OperatingSystems []string `json:"operatingSystems,omitempty"`
	Browsers         []string `json:"browsers,omitempty"`
}

type LiveStatus struct {
	AmountSpent   float64 `json:"amountSpent,omitempty"`
	CampaignOnAir bool    `json:"campaignOnAir"`
	OnAirReason   string  `json:"onAirReason,omitempty"`
}

type Campaign struct {
	ID                 string            `json:"id"`
	MarketerID         string            `json:"marketerId"`
	Name               string            `json:"name"`
	Enabled            bool              `json:"enabled"`
	CPC                float64           `json:"cpc"`
	MinimumCPC         float64           `json:"minimumCpc"`
	Currency           string            `json:"currency"`
	CreationTime       string            `json:"creationTime"`
	LastModified       string            `json:"lastModified"`
	AutoArchived       bool              `json:"autoArchived"`
	Targeting          CampaignTargeting `json:"targeting"`
	Feeds              []string          `json:"feeds,omitempty"`
	ContentType        string            `json:"contentType,omitempty"`
	Budget             Budget            `json:"budget"`
	SuffixTrackingCode string            `json:"suffixTrackingCode,omitempty"`
	LiveStatus         LiveStatus        `json:"liveStatus"`
	BlockedSites       json.RawMessage   `json:"blockedSites,omitempty"`
	Bids               json.RawMessage   `json:"bids,omitempty"`
	OnAirType          string            `json:"onAirType,omitempty"`
	Scheduling         json.RawMessage   `json:"scheduling,omitempty"`
	Objective          string            `json:"objective,omitempty"`
	CreativeFormat     string            `json:"creativeFormat,omitempty"`
}

type ListCampaignsRequest struct {
	IncludeArchived     bool
	FromBudgetStartDate string
	ToBudgetEndDate     string
	Limit               int
	Offset              int
	DaysToLookBack      int
}

type CampaignPage struct {
	Items []Campaign
	Count int
}

type CreateCampaignRequest struct {
	Name               string
	CPC                float64
	BudgetID           string
	Targeting          CampaignTargeting
	SuffixTrackingCode string
	Objective          string
	CreativeFormat     string
}

type UpdateCampaignRequest struct {
	Name               *string  `json:"name,omitempty"`
	CPC                *float64 `json:"cpc,omitempty"`
	SuffixTrackingCode *string  `json:"suffixTrackingCode,omitempty"`
	Objective          *string  `json:"objective,omitempty"`
}

type ApprovalStatus struct {
	Status     string   `json:"status"`
	Reasons    []string `json:"reasons,omitempty"`
	IsEditable bool     `json:"isEditable"`
}

type OnAirStatus struct {
	OnAir  bool   `json:"onAir"`
	Reason string `json:"reason,omitempty"`
}

type ImageMetadata struct {
	ID                string          `json:"id,omitempty"`
	URL               string          `json:"url,omitempty"`
	RequestedImageURL string          `json:"requestedImageUrl,omitempty"`
	OriginalImageURL  string          `json:"originalImageUrl,omitempty"`
	IsAIGenerated     bool            `json:"isAiGenerated,omitempty"`
	CropSettings      json.RawMessage `json:"cropSettings,omitempty"`
}

type PromotedLink struct {
	ID               string          `json:"id"`
	CampaignID       string          `json:"campaignId"`
	Text             string          `json:"text"`
	URL              string          `json:"url"`
	SectionName      string          `json:"sectionName,omitempty"`
	Description      string          `json:"description,omitempty"`
	CreationTime     string          `json:"creationTime"`
	LastModified     string          `json:"lastModified"`
	Status           string          `json:"status,omitempty"`
	ApprovalStatus   ApprovalStatus  `json:"approvalStatus"`
	Enabled          bool            `json:"enabled"`
	Archived         bool            `json:"archived"`
	DocumentLanguage string          `json:"documentLanguage,omitempty"`
	BaseURL          string          `json:"baseUrl,omitempty"`
	DocumentID       string          `json:"documentID,omitempty"`
	CPC              float64         `json:"cpc,omitempty"`
	CPCAdjustment    float64         `json:"cpcAdjustment,omitempty"`
	CallToAction     string          `json:"callToAction,omitempty"`
	ImageMetadata    ImageMetadata   `json:"imageMetadata"`
	MetaData         json.RawMessage `json:"metaData,omitempty"`
	OnAirStatus      OnAirStatus     `json:"onAirStatus"`
}

func (link PromotedLink) Approved() bool {
	return equalFold(link.Status, "APPROVED") || equalFold(link.ApprovalStatus.Status, "APPROVED")
}

type ListPromotedLinksRequest struct {
	Enabled     *bool
	Statuses    []string
	Limit       int
	Offset      int
	Sort        string
	ImageWidth  int
	ImageHeight int
}

type PromotedLinkPage struct {
	Items      []PromotedLink
	Count      int
	TotalCount int
}

type CreatePromotedLinkRequest struct {
	Text        string
	URL         string
	CPC         *float64
	SectionName string
	Description string
	ImageURL    string
	AIGenerated bool
}

type PromotedLinkCPCUpdate struct {
	ID  string  `json:"id"`
	CPC float64 `json:"cpc"`
}

type OperationStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type PromotedLinkUpdateResult struct {
	OperationStatus OperationStatus `json:"operationStatus"`
	ID              string          `json:"id,omitempty"`
	URL             string          `json:"url,omitempty"`
	PromotedLink    PromotedLink    `json:"promotedLink"`
}

type ConversionMetric struct {
	Name               string  `json:"name"`
	TotalConversions   float64 `json:"totalConversions"`
	Conversions        float64 `json:"conversions"`
	ViewConversions    float64 `json:"viewConversions"`
	ConversionRate     float64 `json:"conversionRate"`
	ViewConversionRate float64 `json:"viewConversionRate"`
	TotalCPA           float64 `json:"totalCpa"`
	CPA                float64 `json:"cpa"`
	TotalValue         float64 `json:"totalValue"`
	TotalSumValue      float64 `json:"totalSumValue"`
	SumValue           float64 `json:"sumValue"`
	ViewSumValue       float64 `json:"viewSumValue"`
	TotalAverageValue  float64 `json:"totalAverageValue"`
	AverageValue       float64 `json:"averageValue"`
	ViewAverageValue   float64 `json:"viewAverageValue"`
	ROAS               float64 `json:"roas"`
	TotalROAS          float64 `json:"totalRoas"`
}

type Metrics struct {
	Impressions                      int64              `json:"impressions"`
	ViewableImpressions              int64              `json:"viewableImpressions,omitempty"`
	Clicks                           int64              `json:"clicks"`
	TotalConversions                 float64            `json:"totalConversions"`
	Conversions                      float64            `json:"conversions"`
	ViewConversions                  float64            `json:"viewConversions"`
	Spend                            float64            `json:"spend"`
	ECPC                             float64            `json:"ecpc"`
	CTR                              float64            `json:"ctr"`
	CPM                              float64            `json:"cpm,omitempty"`
	ConversionRate                   float64            `json:"conversionRate"`
	ViewConversionRate               float64            `json:"viewConversionRate"`
	CPA                              float64            `json:"cpa"`
	TotalCPA                         float64            `json:"totalCpa"`
	TotalValue                       float64            `json:"totalValue"`
	TotalSumValue                    float64            `json:"totalSumValue"`
	SumValue                         float64            `json:"sumValue"`
	ViewSumValue                     float64            `json:"viewSumValue"`
	TotalAverageValue                float64            `json:"totalAverageValue"`
	AverageValue                     float64            `json:"averageValue"`
	ViewAverageValue                 float64            `json:"viewAverageValue"`
	ROAS                             float64            `json:"roas"`
	TotalROAS                        float64            `json:"totalRoas"`
	ConversionMetrics                []ConversionMetric `json:"conversionMetrics,omitempty"`
	VideoReachedFirstQ               int64              `json:"videoReachedFirstQ,omitempty"`
	VideoReachedSecondQ              int64              `json:"videoReachedSecondQ,omitempty"`
	VideoReachedThirdQ               int64              `json:"videoReachedThirdQ,omitempty"`
	VideoReachedCompletion           int64              `json:"videoReachedCompletion,omitempty"`
	VideoViewDuration                float64            `json:"videoViewDuration,omitempty"`
	VideoAverageViewDuration         float64            `json:"videoAvgViewDuration,omitempty"`
	VideoPlays                       int64              `json:"videoPlays,omitempty"`
	ClicksOnVideo                    int64              `json:"clicksOnVideo,omitempty"`
	VideoActiveCompletions           int64              `json:"videoActiveCompletions,omitempty"`
	VideoActiveCompletionsPercentage float64            `json:"videoActiveCompletionsPercentage,omitempty"`
	Raw                              json.RawMessage    `json:"-"`
}

func (metrics *Metrics) UnmarshalJSON(data []byte) error {
	type alias Metrics
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*metrics = Metrics(decoded)
	metrics.Raw = append(metrics.Raw[:0], data...)
	return nil
}

type CampaignReportMetadata struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	CampaignOnAir   bool    `json:"campaignOnAir"`
	OnAirReason     string  `json:"onAirReason"`
	Enabled         bool    `json:"enabled"`
	Budget          Budget  `json:"budget"`
	LastCappingTime string  `json:"lastCappingTime,omitempty"`
	CPC             float64 `json:"cpc"`
	CreativeFormat  string  `json:"creativeFormat,omitempty"`
}

type PromotedContentMetadata struct {
	ID             string         `json:"id,omitempty"`
	SequenceID     string         `json:"sequenceId,omitempty"`
	CampaignID     string         `json:"campaignId"`
	CampaignName   string         `json:"campaignName"`
	Enabled        bool           `json:"enabled"`
	CreativeFormat string         `json:"creativeFormat"`
	Title          string         `json:"title"`
	URL            string         `json:"url,omitempty"`
	ImageMetadata  ImageMetadata  `json:"imageMetadata"`
	SectionName    string         `json:"sectionName,omitempty"`
	ApprovalStatus ApprovalStatus `json:"approvalStatus"`
	Delivery       string         `json:"delivery,omitempty"`
	OnAirStatus    OnAirStatus    `json:"onAirStatus"`
	CreationTime   string         `json:"creationTime"`
	LastModified   string         `json:"lastModified"`
}

type CampaignPerformanceRow struct {
	Metadata CampaignReportMetadata `json:"metadata"`
	Metrics  Metrics                `json:"metrics"`
}

type PromotedContentPerformanceRow struct {
	Metadata PromotedContentMetadata `json:"metadata"`
	Metrics  Metrics                 `json:"metrics"`
}

type CampaignReport struct {
	Results              []CampaignPerformanceRow `json:"results"`
	TotalResults         int                      `json:"totalResults"`
	Summary              Metrics                  `json:"summary"`
	TotalFilteredResults int                      `json:"totalFilteredResults,omitempty"`
	SummaryFiltered      Metrics                  `json:"summaryFiltered,omitempty"`
}

type PromotedContentReport struct {
	Results      []PromotedContentPerformanceRow `json:"results"`
	TotalResults int                             `json:"totalResults"`
	Summary      Metrics                         `json:"summary"`
}

type PeriodicMetadata struct {
	ID              string `json:"id"`
	FromDate        string `json:"fromDate"`
	ToDate          string `json:"toDate"`
	LastCappingTime string `json:"lastCappingTime,omitempty"`
}

type PeriodicRow struct {
	Metadata PeriodicMetadata `json:"metadata"`
	Metrics  Metrics          `json:"metrics"`
}

type CampaignPeriodicResult struct {
	CampaignID   string        `json:"campaignId"`
	Results      []PeriodicRow `json:"results"`
	TotalResults int           `json:"totalResults"`
}

type CampaignPeriodicReport struct {
	CampaignResults []CampaignPeriodicResult `json:"campaignResults"`
	TotalCampaigns  int                      `json:"totalCampaigns"`
}

type PromotedLinkPeriodicResult struct {
	PromotedLinkID string        `json:"promotedLinkId"`
	Results        []PeriodicRow `json:"results"`
	TotalResults   int           `json:"totalResults"`
}

type PromotedContentPeriodicReport struct {
	PromotedLinkResults []PromotedLinkPeriodicResult `json:"promotedLinkResults"`
	TotalPromotedLinks  int                          `json:"totalPromotedLinks"`
}

type CampaignReportRequest struct {
	From                     string
	To                       string
	Limit                    int
	Offset                   int
	Sort                     string
	Filter                   string
	IncludeArchivedCampaigns bool
	BudgetID                 string
	CampaignIDs              []string
	IncludeConversionDetails bool
	ConversionsByClickDate   bool
	IncludeViewedImpressions bool
	Timezone                 string
	EnabledCampaignsOnly     bool
}

type PromotedContentReportRequest struct {
	From                     string
	To                       string
	Limit                    int
	Offset                   int
	Sort                     string
	Filter                   string
	IncludeArchivedCampaigns bool
	BudgetID                 string
	CampaignIDs              []string
	PromotedLinkID           string
	SequenceID               string
	IncludeConversionDetails bool
	ConversionsByClickDate   bool
	EnabledCampaignsOnly     bool
}

type CampaignPeriodicReportRequest struct {
	From                     string
	To                       string
	CampaignIDs              []string
	Limit                    int
	Offset                   int
	Filter                   string
	Breakdown                string
	IncludeArchivedCampaigns bool
	IncludeConversionDetails bool
	ConversionsByClickDate   bool
	IncludeViewedImpressions bool
	EnabledCampaignsOnly     bool
}

type PromotedContentPeriodicReportRequest struct {
	CampaignID               string
	From                     string
	To                       string
	Limit                    int
	Offset                   int
	Filter                   string
	Breakdown                string
	IncludeConversionDetails bool
	ConversionsByClickDate   bool
	EnabledCampaignsOnly     bool
}

type MarketerWorkflow interface {
	ListMarketers(context.Context, ...socialhub.CallOption) ([]Marketer, error)
	GetMarketer(context.Context, ...socialhub.CallOption) (Marketer, error)
	ValidateConfiguredMarketer(context.Context, ...socialhub.CallOption) (Marketer, error)
}

type BudgetWorkflow interface {
	ListBudgets(context.Context, bool, ...socialhub.CallOption) ([]Budget, error)
	GetBudget(context.Context, string, ...socialhub.CallOption) (Budget, error)
	CreateBudget(context.Context, CreateBudgetRequest, ...socialhub.CallOption) (Budget, error)
	UpdateBudget(context.Context, string, UpdateBudgetRequest, ...socialhub.CallOption) (Budget, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (CampaignPage, error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
	SetCampaignEnabled(context.Context, string, bool, ...socialhub.CallOption) (Campaign, error)
}

type PromotedLinkWorkflow interface {
	ListPromotedLinks(context.Context, string, ListPromotedLinksRequest, ...socialhub.CallOption) (PromotedLinkPage, error)
	GetPromotedLink(context.Context, string, string, ...socialhub.CallOption) (PromotedLink, error)
	CreatePromotedLink(context.Context, string, CreatePromotedLinkRequest, ...socialhub.CallOption) (PromotedLink, error)
	SetPromotedLinkEnabled(context.Context, string, string, bool, ...socialhub.CallOption) error
	UpdatePromotedLinkCPCs(context.Context, string, []PromotedLinkCPCUpdate, ...socialhub.CallOption) ([]PromotedLinkUpdateResult, error)
}

type ReportWorkflow interface {
	CampaignPerformance(context.Context, CampaignReportRequest, ...socialhub.CallOption) (CampaignReport, error)
	PromotedContentPerformance(context.Context, PromotedContentReportRequest, ...socialhub.CallOption) (PromotedContentReport, error)
	CampaignPeriodicPerformance(context.Context, CampaignPeriodicReportRequest, ...socialhub.CallOption) (CampaignPeriodicReport, error)
	PromotedContentPeriodicPerformance(context.Context, PromotedContentPeriodicReportRequest, ...socialhub.CallOption) (PromotedContentPeriodicReport, error)
}

func equalFold(left, right string) bool {
	return strings.EqualFold(left, right)
}
