package cm360

import (
	"context"
	"io"

	"social-hub/pkg/socialhub"
)

type SortOrder string

const (
	SortAscending  SortOrder = "ASCENDING"
	SortDescending SortOrder = "DESCENDING"
)

type CampaignSortField string

const (
	CampaignSortID   CampaignSortField = "ID"
	CampaignSortName CampaignSortField = "NAME"
)

type PlacementActiveStatus string

const (
	PlacementUnknown             PlacementActiveStatus = "PLACEMENT_STATUS_UNKNOWN"
	PlacementActive              PlacementActiveStatus = "PLACEMENT_STATUS_ACTIVE"
	PlacementInactive            PlacementActiveStatus = "PLACEMENT_STATUS_INACTIVE"
	PlacementArchived            PlacementActiveStatus = "PLACEMENT_STATUS_ARCHIVED"
	PlacementPermanentlyArchived PlacementActiveStatus = "PLACEMENT_STATUS_PERMANENTLY_ARCHIVED"
)

type AdType string

const (
	AdStandard     AdType = "AD_SERVING_STANDARD_AD"
	AdDefault      AdType = "AD_SERVING_DEFAULT_AD"
	AdClickTracker AdType = "AD_SERVING_CLICK_TRACKER"
	AdTracking     AdType = "AD_SERVING_TRACKING"
	AdBrandSafe    AdType = "AD_SERVING_BRAND_SAFE_AD"
)

type ReportScope string

const (
	ReportScopeAll          ReportScope = "ALL"
	ReportScopeMine         ReportScope = "MINE"
	ReportScopeSharedWithMe ReportScope = "SHARED_WITH_ME"
)

type ReportFileStatus string

const (
	ReportFileProcessing ReportFileStatus = "PROCESSING"
	ReportFileAvailable  ReportFileStatus = "REPORT_AVAILABLE"
	ReportFileFailed     ReportFileStatus = "FAILED"
	ReportFileCancelled  ReportFileStatus = "CANCELLED"
	ReportFileQueued     ReportFileStatus = "QUEUED"
)

type UserProfile struct {
	ProfileID      string `json:"profileId"`
	AccountID      string `json:"accountId"`
	AccountName    string `json:"accountName"`
	SubaccountID   string `json:"subAccountId,omitempty"`
	SubaccountName string `json:"subAccountName,omitempty"`
	UserName       string `json:"userName"`
}

type Advertiser struct {
	ID                        string `json:"id"`
	AccountID                 string `json:"accountId"`
	SubaccountID              string `json:"subaccountId,omitempty"`
	Name                      string `json:"name"`
	Status                    string `json:"status"`
	Suspended                 bool   `json:"suspended"`
	FloodlightConfigurationID string `json:"floodlightConfigurationId,omitempty"`
}

type LastModifiedInfo struct {
	Time string `json:"time,omitempty"`
}

type Campaign struct {
	ID                 string            `json:"id"`
	AccountID          string            `json:"accountId,omitempty"`
	SubaccountID       string            `json:"subaccountId,omitempty"`
	AdvertiserID       string            `json:"advertiserId"`
	Name               string            `json:"name"`
	Archived           bool              `json:"archived"`
	StartDate          string            `json:"startDate,omitempty"`
	EndDate            string            `json:"endDate,omitempty"`
	Comment            string            `json:"comment,omitempty"`
	BillingInvoiceCode string            `json:"billingInvoiceCode,omitempty"`
	LastModifiedInfo   *LastModifiedInfo `json:"lastModifiedInfo,omitempty"`
}

type Size struct {
	ID     string `json:"id,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type Placement struct {
	ID               string                `json:"id"`
	AccountID        string                `json:"accountId,omitempty"`
	SubaccountID     string                `json:"subaccountId,omitempty"`
	AdvertiserID     string                `json:"advertiserId"`
	CampaignID       string                `json:"campaignId"`
	SiteID           string                `json:"siteId,omitempty"`
	Name             string                `json:"name"`
	ActiveStatus     PlacementActiveStatus `json:"activeStatus"`
	Status           string                `json:"status,omitempty"`
	Compatibility    string                `json:"compatibility,omitempty"`
	PaymentSource    string                `json:"paymentSource,omitempty"`
	StartDate        string                `json:"startDate,omitempty"`
	EndDate          string                `json:"endDate,omitempty"`
	Size             *Size                 `json:"size,omitempty"`
	LastModifiedInfo *LastModifiedInfo     `json:"lastModifiedInfo,omitempty"`
}

type Ad struct {
	ID               string            `json:"id"`
	AccountID        string            `json:"accountId,omitempty"`
	SubaccountID     string            `json:"subaccountId,omitempty"`
	AdvertiserID     string            `json:"advertiserId"`
	CampaignID       string            `json:"campaignId"`
	Name             string            `json:"name"`
	Type             AdType            `json:"type"`
	Active           bool              `json:"active"`
	Archived         bool              `json:"archived"`
	Compatibility    string            `json:"compatibility,omitempty"`
	StartTime        string            `json:"startTime,omitempty"`
	EndTime          string            `json:"endTime,omitempty"`
	Size             *Size             `json:"size,omitempty"`
	LastModifiedInfo *LastModifiedInfo `json:"lastModifiedInfo,omitempty"`
}

type Page[T any] struct {
	Items         []T
	NextPageToken string
}

type CampaignListRequest struct {
	MaxResults   int
	PageToken    string
	SearchString string
	Archived     *bool
	IDs          []string
	SortField    CampaignSortField
	SortOrder    SortOrder
}

type PlacementListRequest struct {
	MaxResults   int
	PageToken    string
	SearchString string
	CampaignIDs  []string
	ActiveStatus PlacementActiveStatus
	SortOrder    SortOrder
}

type AdListRequest struct {
	MaxResults   int
	PageToken    string
	SearchString string
	CampaignIDs  []string
	Active       *bool
	Archived     *bool
	Type         AdType
	SortOrder    SortOrder
}

type CreateCampaignRequest struct {
	Name               string
	StartDate          string
	EndDate            string
	Comment            string
	BillingInvoiceCode string
}

type UpdateCampaignRequest struct {
	Name               *string
	Archived           *bool
	StartDate          *string
	EndDate            *string
	Comment            *string
	BillingInvoiceCode *string
}

type Report struct {
	ID               string `json:"id"`
	AccountID        string `json:"accountId,omitempty"`
	OwnerProfileID   string `json:"ownerProfileId,omitempty"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	FileName         string `json:"fileName,omitempty"`
	Format           string `json:"format,omitempty"`
	LastModifiedTime string `json:"lastModifiedTime,omitempty"`
}

type DateRange struct {
	StartDate         string `json:"startDate,omitempty"`
	EndDate           string `json:"endDate,omitempty"`
	RelativeDateRange string `json:"relativeDateRange,omitempty"`
}

type DimensionValue struct {
	DimensionName string `json:"dimensionName"`
	ID            string `json:"id,omitempty"`
	Value         string `json:"value,omitempty"`
	MatchType     string `json:"matchType,omitempty"`
}

type SortBy struct {
	Name      string    `json:"name"`
	SortOrder SortOrder `json:"sortOrder,omitempty"`
}

type ReportDataQueryRequest struct {
	DateRange        DateRange        `json:"dateRange"`
	DimensionNames   []string         `json:"dimensionNames,omitempty"`
	MetricNames      []string         `json:"metricNames"`
	DimensionFilters []DimensionValue `json:"dimensionFilters,omitempty"`
	SortBys          []SortBy         `json:"sortBys,omitempty"`
	MaxResults       int              `json:"maxResults,omitempty"`
	PageToken        string           `json:"pageToken,omitempty"`
}

type ColumnHeader struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ReportDataRow struct {
	Values []string `json:"values"`
}

type ReportDataResponse struct {
	ColumnHeaders []ColumnHeader  `json:"columnHeaders"`
	Rows          []ReportDataRow `json:"rows"`
	TotalRow      *ReportDataRow  `json:"totalRow,omitempty"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
}

type ReportListRequest struct {
	MaxResults int
	PageToken  string
	Scope      ReportScope
	SortOrder  SortOrder
}

type ReportFile struct {
	ID               string           `json:"id"`
	ReportID         string           `json:"reportId"`
	FileName         string           `json:"fileName,omitempty"`
	Format           string           `json:"format,omitempty"`
	Status           ReportFileStatus `json:"status"`
	DateRange        DateRange        `json:"dateRange,omitempty"`
	LastModifiedTime string           `json:"lastModifiedTime,omitempty"`
}

type ReportFileListRequest struct {
	MaxResults int
	PageToken  string
	SortOrder  SortOrder
}

// ByteRange is an inclusive HTTP byte range. Each request is capped at 8 MiB.
type ByteRange struct {
	Start int64
	End   int64
}

type DownloadResult struct {
	BytesWritten int
	ContentRange string
	Complete     bool
}

type ProfileWorkflow interface {
	GetProfile(context.Context, ...socialhub.CallOption) (UserProfile, error)
}

type AdvertiserWorkflow interface {
	GetAdvertiser(context.Context, ...socialhub.CallOption) (Advertiser, error)
}

type CampaignWorkflow interface {
	GetCampaign(context.Context, string, ...socialhub.CallOption) (Campaign, error)
	ListCampaigns(context.Context, CampaignListRequest, ...socialhub.CallOption) (Page[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (Campaign, error)
}

type PlacementWorkflow interface {
	GetPlacement(context.Context, string, ...socialhub.CallOption) (Placement, error)
	ListPlacements(context.Context, PlacementListRequest, ...socialhub.CallOption) (Page[Placement], error)
}

type AdWorkflow interface {
	GetAd(context.Context, string, ...socialhub.CallOption) (Ad, error)
	ListAds(context.Context, AdListRequest, ...socialhub.CallOption) (Page[Ad], error)
}

type ReportingWorkflow interface {
	QueryReportData(context.Context, ReportDataQueryRequest, ...socialhub.CallOption) (ReportDataResponse, error)
	GetReport(context.Context, string, ...socialhub.CallOption) (Report, error)
	ListReports(context.Context, ReportListRequest, ...socialhub.CallOption) (Page[Report], error)
	RunReport(context.Context, string, bool, ...socialhub.CallOption) (ReportFile, error)
	GetReportFile(context.Context, string, string, ...socialhub.CallOption) (ReportFile, error)
	ListReportFiles(context.Context, string, ReportFileListRequest, ...socialhub.CallOption) (Page[ReportFile], error)
	DownloadReportFileRange(context.Context, string, string, ByteRange, io.Writer, ...socialhub.CallOption) (DownloadResult, error)
}

type listCampaignsResponse struct {
	Campaigns     []Campaign `json:"campaigns"`
	NextPageToken string     `json:"nextPageToken"`
}

type listPlacementsResponse struct {
	Placements    []Placement `json:"placements"`
	NextPageToken string      `json:"nextPageToken"`
}

type listAdsResponse struct {
	Ads           []Ad   `json:"ads"`
	NextPageToken string `json:"nextPageToken"`
}

type listReportsResponse struct {
	Items         []Report `json:"items"`
	NextPageToken string   `json:"nextPageToken"`
}

type listReportFilesResponse struct {
	Items         []ReportFile `json:"items"`
	NextPageToken string       `json:"nextPageToken"`
}
