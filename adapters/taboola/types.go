package taboola

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type CampaignStatus string

const (
	CampaignRunning          CampaignStatus = "RUNNING"
	CampaignPaused           CampaignStatus = "PAUSED"
	CampaignPendingStartDate CampaignStatus = "PENDING_START_DATE"
	CampaignDepletedMonthly  CampaignStatus = "DEPLETED_MONTHLY"
	CampaignDepleted         CampaignStatus = "DEPLETED"
	CampaignExpired          CampaignStatus = "EXPIRED"
	CampaignTerminated       CampaignStatus = "TERMINATED"
	CampaignFrozen           CampaignStatus = "FROZEN"
	CampaignPendingApproval  CampaignStatus = "PENDING_APPROVAL"
	CampaignRejected         CampaignStatus = "REJECTED"
)

type ItemStatus string

const (
	ItemRunning         ItemStatus = "RUNNING"
	ItemCrawling        ItemStatus = "CRAWLING"
	ItemCrawlingError   ItemStatus = "CRAWLING_ERROR"
	ItemNeedToEdit      ItemStatus = "NEED_TO_EDIT"
	ItemPaused          ItemStatus = "PAUSED"
	ItemStopped         ItemStatus = "STOPPED"
	ItemPendingApproval ItemStatus = "PENDING_APPROVAL"
	ItemRejected        ItemStatus = "REJECTED"
	ItemFailedToCreate  ItemStatus = "FAILED_TO_CREATE"
)

type BidStrategy string

const (
	BidStrategyFixed          BidStrategy = "FIXED"
	BidStrategyMaxConversions BidStrategy = "MAX_CONVERSIONS"
)

type SpendingLimitModel string

const (
	SpendingNone    SpendingLimitModel = "NONE"
	SpendingMonthly SpendingLimitModel = "MONTHLY"
	SpendingEntire  SpendingLimitModel = "ENTIRE"
)

const (
	FetchRecent          = "R"
	FetchRecentAndPaused = "RAP"
)

// Account is one account returned by current-account or allowed-accounts.
type Account struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	AccountID     string   `json:"account_id"`
	PartnerTypes  []string `json:"partner_types"`
	Type          string   `json:"type"`
	CampaignTypes []string `json:"campaign_types"`
	Currency      string   `json:"currency"`
	TimeZoneName  string   `json:"time_zone_name"`
}

// Campaign contains the core Backstage fields used by the initial adapter.
type Campaign struct {
	ID                 string             `json:"id"`
	AdvertiserID       string             `json:"advertiser_id"`
	Name               string             `json:"name"`
	BrandingText       string             `json:"branding_text"`
	BidStrategy        BidStrategy        `json:"bid_strategy"`
	MarketingObjective string             `json:"marketing_objective"`
	CPC                *float64           `json:"cpc"`
	DailyCap           *float64           `json:"daily_cap"`
	SpendingLimit      *float64           `json:"spending_limit"`
	SpendingLimitModel SpendingLimitModel `json:"spending_limit_model"`
	StartDate          string             `json:"start_date"`
	EndDate            string             `json:"end_date"`
	ApprovalState      string             `json:"approval_state"`
	IsActive           *bool              `json:"is_active"`
	Status             CampaignStatus     `json:"status"`
	Spent              *float64           `json:"spent"`
}

// CampaignItem is a static creative discovered from a destination URL.
type CampaignItem struct {
	ID            string     `json:"id"`
	CampaignID    string     `json:"campaign_id"`
	Type          string     `json:"type"`
	URL           string     `json:"url"`
	ThumbnailURL  string     `json:"thumbnail_url"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	ApprovalState string     `json:"approval_state"`
	IsActive      *bool      `json:"is_active"`
	Status        ItemStatus `json:"status"`
}

type ListCampaignsRequest struct {
	FetchLevel string
	Page       int
	PageSize   int
	Sort       string
}

type CampaignPage struct {
	Items    []Campaign
	Page     int
	PageSize int
	Total    int
	Count    int
	HasMore  bool
}

type CreateCampaignRequest struct {
	Name               string
	BrandingText       string
	BidStrategy        BidStrategy
	MarketingObjective string
	CPC                *float64
	DailyCap           *float64
	SpendingLimit      *float64
	SpendingLimitModel SpendingLimitModel
	StartDate          string
	EndDate            string
}

type UpdateCampaignRequest struct {
	Name               *string
	BrandingText       *string
	MarketingObjective *string
	CPC                *float64
	DailyCap           *float64
	SpendingLimit      *float64
	SpendingLimitModel *SpendingLimitModel
	StartDate          *string
	EndDate            *string
}

type CreateItemRequest struct {
	URL string
}

type UpdateItemRequest struct {
	URL          *string
	ThumbnailURL *string
	Title        *string
	Description  *string
}

type ReportRequest struct {
	Dimension               string
	StartDate               string
	EndDate                 string
	CampaignIDs             []string
	Platform                string
	Country                 string
	Site                    string
	PartnerName             string
	IncludeMultiConversions bool
}

type RealtimeReportRequest struct {
	Dimension   string
	StartDate   string
	EndDate     string
	CampaignIDs []string
	Platform    string
	Country     string
	SiteID      string
	FetchConfig bool
}

// ReportRow preserves the exact platform row while exposing common metrics.
type ReportRow struct {
	CampaignID         string          `json:"campaign_id"`
	CampaignName       string          `json:"campaign_name"`
	Campaign           string          `json:"campaign"`
	Date               string          `json:"date"`
	Hour               string          `json:"hour"`
	Impressions        int64           `json:"impressions"`
	VisibleImpressions int64           `json:"visible_impressions"`
	Clicks             int64           `json:"clicks"`
	Spent              float64         `json:"spent"`
	CTR                float64         `json:"ctr"`
	CPC                float64         `json:"cpc"`
	CPM                float64         `json:"cpm"`
	ConversionsValue   float64         `json:"conversions_value"`
	ROAS               float64         `json:"roas"`
	Currency           string          `json:"currency"`
	Raw                json.RawMessage `json:"-"`
}

func (row *ReportRow) UnmarshalJSON(data []byte) error {
	type alias ReportRow
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.Raw = append(json.RawMessage(nil), data...)
	*row = ReportRow(decoded)
	return nil
}

type ReportResult struct {
	Rows        []ReportRow
	RecordCount int
	Total       int
	Count       int
	Timezone    string
}

type AccountWorkflow interface {
	CurrentAccount(context.Context, ...socialhub.CallOption) (*Account, error)
	AllowedAccounts(context.Context, ...socialhub.CallOption) ([]Account, error)
	ValidateConfiguredAccount(context.Context, ...socialhub.CallOption) (*Account, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (CampaignPage, error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignActive(context.Context, string, bool, ...socialhub.CallOption) (*Campaign, error)
}

type ItemWorkflow interface {
	ListItems(context.Context, string, ...socialhub.CallOption) ([]CampaignItem, error)
	GetItem(context.Context, string, string, ...socialhub.CallOption) (*CampaignItem, error)
	CreateItem(context.Context, string, CreateItemRequest, ...socialhub.CallOption) (*CampaignItem, error)
	UpdateItem(context.Context, string, string, UpdateItemRequest, ...socialhub.CallOption) (*CampaignItem, error)
	SetItemActive(context.Context, string, string, bool, ...socialhub.CallOption) (*CampaignItem, error)
}

type ReportWorkflow interface {
	CampaignSummaryReport(context.Context, ReportRequest, ...socialhub.CallOption) (ReportResult, error)
	RealtimeCampaignReport(context.Context, RealtimeReportRequest, ...socialhub.CallOption) (ReportResult, error)
}
