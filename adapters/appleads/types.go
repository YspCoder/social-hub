package appleads

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type CampaignStatus string
type AdGroupStatus string
type KeywordStatus string
type AdStatus string
type MatchType string
type SortOrder string
type Granularity string
type TimeZone string

const (
	CampaignEnabled CampaignStatus = "ENABLED"
	CampaignPaused  CampaignStatus = "PAUSED"

	AdGroupEnabled AdGroupStatus = "ENABLED"
	AdGroupPaused  AdGroupStatus = "PAUSED"

	KeywordActive KeywordStatus = "ACTIVE"
	KeywordPaused KeywordStatus = "PAUSED"

	AdEnabled AdStatus = "ENABLED"
	AdPaused  AdStatus = "PAUSED"

	MatchBroad MatchType = "BROAD"
	MatchExact MatchType = "EXACT"

	SortAscending  SortOrder = "ASCENDING"
	SortDescending SortOrder = "DESCENDING"

	GranularityHourly  Granularity = "HOURLY"
	GranularityDaily   Granularity = "DAILY"
	GranularityWeekly  Granularity = "WEEKLY"
	GranularityMonthly Granularity = "MONTHLY"

	TimeZoneUTC          TimeZone = "UTC"
	TimeZoneOrganization TimeZone = "ORTZ"

	CreativeCustomProductPage  = "CUSTOM_PRODUCT_PAGE"
	CreativeDefaultProductPage = "DEFAULT_PRODUCT_PAGE"
)

// Money preserves Apple's decimal amount representation without float rounding.
type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type Pagination struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type PageDetail struct {
	TotalResults int64 `json:"totalResults"`
	StartIndex   int   `json:"startIndex"`
	ItemsPerPage int   `json:"itemsPerPage"`
}

type Page[T any] struct {
	Items      []T
	Pagination PageDetail
	HasMore    bool
}

type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Values   []any  `json:"values"`
}

type Sorting struct {
	Field     string    `json:"field"`
	SortOrder SortOrder `json:"sortOrder"`
}

type Selector struct {
	Conditions []Condition `json:"conditions,omitempty"`
	Fields     []string    `json:"fields,omitempty"`
	OrderBy    []Sorting   `json:"orderBy,omitempty"`
	Pagination Pagination  `json:"pagination"`
}

type UserACL struct {
	OrgID        int64    `json:"orgId"`
	OrgName      string   `json:"orgName"`
	ParentOrgID  int64    `json:"parentOrgId"`
	Currency     string   `json:"currency"`
	PaymentModel string   `json:"paymentModel"`
	RoleNames    []string `json:"roleNames"`
	TimeZone     string   `json:"timeZone"`
}

type Campaign struct {
	ID                                 int64               `json:"id"`
	OrgID                              int64               `json:"orgId"`
	Name                               string              `json:"name"`
	AdamID                             int64               `json:"adamId"`
	DailyBudgetAmount                  Money               `json:"dailyBudgetAmount"`
	BudgetAmount                       *Money              `json:"budgetAmount,omitempty"`
	BillingEvent                       string              `json:"billingEvent"`
	SupplySources                      []string            `json:"supplySources"`
	CountriesOrRegions                 []string            `json:"countriesOrRegions"`
	AdChannelType                      string              `json:"adChannelType"`
	BiddingStrategy                    string              `json:"biddingStrategy"`
	TargetCPA                          *Money              `json:"targetCpa,omitempty"`
	PaymentModel                       string              `json:"paymentModel,omitempty"`
	Status                             CampaignStatus      `json:"status"`
	ServingStatus                      string              `json:"servingStatus"`
	DisplayStatus                      string              `json:"displayStatus"`
	ServingStateReasons                []string            `json:"servingStateReasons,omitempty"`
	CountryOrRegionServingStateReasons map[string][]string `json:"countryOrRegionServingStateReasons,omitempty"`
	StartTime                          string              `json:"startTime,omitempty"`
	EndTime                            string              `json:"endTime,omitempty"`
	CreationTime                       string              `json:"creationTime,omitempty"`
	ModificationTime                   string              `json:"modificationTime,omitempty"`
	Deleted                            bool                `json:"deleted"`
}

type CreateCampaignRequest struct {
	Name               string
	AdamID             int64
	DailyBudgetAmount  Money
	BudgetAmount       *Money
	BillingEvent       string
	SupplySources      []string
	CountriesOrRegions []string
	AdChannelType      string
	BiddingStrategy    string
	TargetCPA          *Money
	StartTime          string
	EndTime            string
}

type UpdateCampaignRequest struct {
	Name                             *string
	DailyBudgetAmount                *Money
	BudgetAmount                     *Money
	CountriesOrRegions               []string
	BiddingStrategy                  *string
	TargetCPA                        *Money
	StartTime                        *string
	EndTime                          *string
	ClearGeoTargetingOnCountryChange *bool
}

type AdGroup struct {
	ID                        int64           `json:"id"`
	OrgID                     int64           `json:"orgId"`
	CampaignID                int64           `json:"campaignId"`
	Name                      string          `json:"name"`
	CPAGoal                   *Money          `json:"cpaGoal,omitempty"`
	DefaultBidAmount          *Money          `json:"defaultBidAmount,omitempty"`
	PricingModel              string          `json:"pricingModel"`
	AutomatedKeywordsOptIn    bool            `json:"automatedKeywordsOptIn"`
	AutomatedKeywordsRequired bool            `json:"automatedKeywordsRequired,omitempty"`
	TargetingDimensions       json.RawMessage `json:"targetingDimensions,omitempty"`
	BiddingStrategy           string          `json:"biddingStrategy,omitempty"`
	Status                    AdGroupStatus   `json:"status"`
	ServingStatus             string          `json:"servingStatus"`
	DisplayStatus             string          `json:"displayStatus"`
	ServingStateReasons       []string        `json:"servingStateReasons,omitempty"`
	StartTime                 string          `json:"startTime,omitempty"`
	EndTime                   string          `json:"endTime,omitempty"`
	CreationTime              string          `json:"creationTime,omitempty"`
	ModificationTime          string          `json:"modificationTime,omitempty"`
	Deleted                   bool            `json:"deleted"`
}

type CreateAdGroupRequest struct {
	Name                      string
	CPAGoal                   *Money
	DefaultBidAmount          *Money
	PricingModel              string
	AutomatedKeywordsOptIn    bool
	AutomatedKeywordsRequired bool
	TargetingDimensions       json.RawMessage
	StartTime                 string
	EndTime                   string
}

type UpdateAdGroupRequest struct {
	Name                   *string
	CPAGoal                *Money
	DefaultBidAmount       *Money
	AutomatedKeywordsOptIn *bool
	TargetingDimensions    json.RawMessage
	StartTime              *string
	EndTime                *string
}

type Keyword struct {
	ID               int64         `json:"id"`
	CampaignID       int64         `json:"campaignId,omitempty"`
	AdGroupID        int64         `json:"adGroupId"`
	Text             string        `json:"text"`
	MatchType        MatchType     `json:"matchType"`
	BidAmount        *Money        `json:"bidAmount,omitempty"`
	Status           KeywordStatus `json:"status"`
	CreationTime     string        `json:"creationTime,omitempty"`
	ModificationTime string        `json:"modificationTime,omitempty"`
	Deleted          bool          `json:"deleted"`
}

type CreateKeywordRequest struct {
	Text      string
	MatchType MatchType
	BidAmount *Money
}

type UpdateKeywordRequest struct {
	ID        int64
	BidAmount *Money
	Status    *KeywordStatus
}

type Creative struct {
	ID               int64    `json:"id"`
	OrgID            int64    `json:"orgId"`
	AdamID           int64    `json:"adamId"`
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	ProductPageID    string   `json:"productPageId,omitempty"`
	State            string   `json:"state"`
	StateReasons     []string `json:"stateReasons,omitempty"`
	CreationTime     string   `json:"creationTime,omitempty"`
	ModificationTime string   `json:"modificationTime,omitempty"`
}

type CreateCreativeRequest struct {
	AdamID        int64
	Name          string
	Type          string
	ProductPageID string
}

type Ad struct {
	ID                  int64    `json:"id"`
	OrgID               int64    `json:"orgId"`
	CampaignID          int64    `json:"campaignId"`
	AdGroupID           int64    `json:"adGroupId"`
	CreativeID          int64    `json:"creativeId"`
	Name                string   `json:"name"`
	CreativeType        string   `json:"creativeType"`
	Status              AdStatus `json:"status"`
	ServingStatus       string   `json:"servingStatus"`
	ServingStateReasons []string `json:"servingStateReasons,omitempty"`
	Deleted             bool     `json:"deleted"`
	CreationTime        string   `json:"creationTime,omitempty"`
	ModificationTime    string   `json:"modificationTime,omitempty"`
}

type CreateAdRequest struct {
	CreativeID int64
	Name       string
}

type UpdateAdRequest struct {
	Name *string
}

type ReportingRequest struct {
	StartTime                  string      `json:"startTime"`
	EndTime                    string      `json:"endTime"`
	Selector                   Selector    `json:"selector"`
	Granularity                Granularity `json:"granularity,omitempty"`
	GroupBy                    []string    `json:"groupBy,omitempty"`
	TimeZone                   TimeZone    `json:"timeZone,omitempty"`
	ReturnRecordsWithNoMetrics bool        `json:"returnRecordsWithNoMetrics"`
	ReturnRowTotals            bool        `json:"returnRowTotals"`
	ReturnGrandTotals          bool        `json:"returnGrandTotals"`
}

type SpendMetrics struct {
	Date              string  `json:"date,omitempty"`
	Impressions       int64   `json:"impressions"`
	Taps              int64   `json:"taps"`
	TapInstalls       int64   `json:"tapInstalls"`
	ViewInstalls      int64   `json:"viewInstalls"`
	TotalInstalls     int64   `json:"totalInstalls"`
	TapNewDownloads   int64   `json:"tapNewDownloads"`
	ViewNewDownloads  int64   `json:"viewNewDownloads"`
	TotalNewDownloads int64   `json:"totalNewDownloads"`
	TapRedownloads    int64   `json:"tapRedownloads"`
	ViewRedownloads   int64   `json:"viewRedownloads"`
	TotalRedownloads  int64   `json:"totalRedownloads"`
	TTR               float64 `json:"ttr"`
	TapInstallRate    float64 `json:"tapInstallRate"`
	TotalInstallRate  float64 `json:"totalInstallRate"`
	LocalSpend        *Money  `json:"localSpend,omitempty"`
	AvgCPT            *Money  `json:"avgCPT,omitempty"`
	AvgCPM            *Money  `json:"avgCPM,omitempty"`
	TapInstallCPI     *Money  `json:"tapInstallCPI,omitempty"`
	TotalAvgCPI       *Money  `json:"totalAvgCPI,omitempty"`
}

type ReportMetadata struct {
	OrgID           int64  `json:"orgId,omitempty"`
	CampaignID      int64  `json:"campaignId,omitempty"`
	CampaignName    string `json:"campaignName,omitempty"`
	AdGroupID       int64  `json:"adGroupId,omitempty"`
	AdGroupName     string `json:"adGroupName,omitempty"`
	KeywordID       int64  `json:"keywordId,omitempty"`
	Keyword         string `json:"keyword,omitempty"`
	AdID            int64  `json:"adId,omitempty"`
	AdName          string `json:"adName,omitempty"`
	CreativeID      int64  `json:"creativeId,omitempty"`
	CreativeType    string `json:"creativeType,omitempty"`
	CountryOrRegion string `json:"countryOrRegion,omitempty"`
}

type ReportRow struct {
	Other       bool           `json:"other"`
	Metadata    ReportMetadata `json:"metadata"`
	Total       *SpendMetrics  `json:"total,omitempty"`
	Granularity []SpendMetrics `json:"granularity,omitempty"`
}

type Report struct {
	Rows        []ReportRow
	GrandTotals *SpendMetrics
	Pagination  PageDetail
}

type ACLWorkflow interface {
	ListACL(context.Context, Pagination, ...socialhub.CallOption) (Page[UserACL], error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, Pagination, ...socialhub.CallOption) (Page[Campaign], error)
	FindCampaigns(context.Context, Selector, ...socialhub.CallOption) (Page[Campaign], error)
	GetCampaign(context.Context, int64, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, int64, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignEnabled(context.Context, int64, bool, ...socialhub.CallOption) (*Campaign, error)
	DeleteCampaign(context.Context, int64, ...socialhub.CallOption) error
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, int64, Pagination, ...socialhub.CallOption) (Page[AdGroup], error)
	GetAdGroup(context.Context, int64, int64, ...socialhub.CallOption) (*AdGroup, error)
	CreateAdGroup(context.Context, int64, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, int64, int64, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupEnabled(context.Context, int64, int64, bool, ...socialhub.CallOption) (*AdGroup, error)
	DeleteAdGroup(context.Context, int64, int64, ...socialhub.CallOption) error
}

type KeywordWorkflow interface {
	ListKeywords(context.Context, int64, int64, Pagination, ...socialhub.CallOption) (Page[Keyword], error)
	GetKeyword(context.Context, int64, int64, int64, ...socialhub.CallOption) (*Keyword, error)
	CreateKeywords(context.Context, int64, int64, []CreateKeywordRequest, ...socialhub.CallOption) ([]Keyword, error)
	UpdateKeywords(context.Context, int64, int64, []UpdateKeywordRequest, ...socialhub.CallOption) ([]Keyword, error)
	DeleteKeyword(context.Context, int64, int64, int64, ...socialhub.CallOption) error
}

type CreativeWorkflow interface {
	ListCreatives(context.Context, Pagination, ...socialhub.CallOption) (Page[Creative], error)
	GetCreative(context.Context, int64, ...socialhub.CallOption) (*Creative, error)
	CreateCreative(context.Context, CreateCreativeRequest, ...socialhub.CallOption) (*Creative, error)
}

type AdWorkflow interface {
	ListAds(context.Context, int64, int64, Pagination, ...socialhub.CallOption) (Page[Ad], error)
	GetAd(context.Context, int64, int64, int64, ...socialhub.CallOption) (*Ad, error)
	CreateAd(context.Context, int64, int64, CreateAdRequest, ...socialhub.CallOption) (*Ad, error)
	UpdateAd(context.Context, int64, int64, int64, UpdateAdRequest, ...socialhub.CallOption) (*Ad, error)
	SetAdEnabled(context.Context, int64, int64, int64, bool, ...socialhub.CallOption) (*Ad, error)
	DeleteAd(context.Context, int64, int64, int64, ...socialhub.CallOption) error
}

type ReportWorkflow interface {
	CampaignReport(context.Context, ReportingRequest, ...socialhub.CallOption) (*Report, error)
	AdGroupReport(context.Context, int64, ReportingRequest, ...socialhub.CallOption) (*Report, error)
	KeywordReport(context.Context, int64, ReportingRequest, ...socialhub.CallOption) (*Report, error)
	AdReport(context.Context, int64, ReportingRequest, ...socialhub.CallOption) (*Report, error)
}
