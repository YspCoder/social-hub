package amazonads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const (
	campaignMediaType     = "application/vnd.spCampaign.v3+json"
	adGroupMediaType      = "application/vnd.spAdGroup.v3+json"
	productAdMediaType    = "application/vnd.spProductAd.v3+json"
	keywordMediaType      = "application/vnd.spKeyword.v3+json"
	reportCreateMediaType = "application/vnd.createasyncreportrequest.v3+json"
)

// Decimal is a base-10 amount encoded as a JSON number. A string-backed type
// avoids float64 rounding while preserving Amazon's wire contract.
type Decimal string

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(string(value), false) {
		return nil, fmt.Errorf("amazonads: invalid decimal")
	}
	return []byte(value), nil
}

func (value *Decimal) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*value = ""
		return nil
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		data = []byte(text)
	}
	if !validDecimal(string(data), false) {
		return fmt.Errorf("amazonads: invalid decimal")
	}
	*value = Decimal(data)
	return nil
}

type State string

const (
	StateEnabled  State = "ENABLED"
	StatePaused   State = "PAUSED"
	StateProposed State = "PROPOSED"
)

type TargetingType string

const (
	TargetingAuto   TargetingType = "AUTO"
	TargetingManual TargetingType = "MANUAL"
)

type MatchType string

const (
	MatchBroad  MatchType = "BROAD"
	MatchPhrase MatchType = "PHRASE"
	MatchExact  MatchType = "EXACT"
)

type BiddingStrategy string

const (
	BiddingLegacyForSales BiddingStrategy = "LEGACY_FOR_SALES"
	BiddingAutoForSales   BiddingStrategy = "AUTO_FOR_SALES"
	BiddingManual         BiddingStrategy = "MANUAL"
	BiddingRuleBased      BiddingStrategy = "RULE_BASED"
)

type Budget struct {
	Type   string  `json:"budgetType"`
	Amount Decimal `json:"budget"`
}

type PlacementBid struct {
	Placement  string `json:"placement"`
	Percentage int    `json:"percentage"`
}

type DynamicBidding struct {
	Strategy         BiddingStrategy `json:"strategy"`
	PlacementBidding []PlacementBid  `json:"placementBidding,omitempty"`
}

type AccountInfo struct {
	MarketplaceStringID string `json:"marketplaceStringId,omitempty"`
	ID                  string `json:"id,omitempty"`
	Type                string `json:"type,omitempty"`
	Name                string `json:"name,omitempty"`
	ValidPaymentMethod  bool   `json:"validPaymentMethod,omitempty"`
}

type Profile struct {
	ID           string      `json:"-"`
	CountryCode  string      `json:"countryCode,omitempty"`
	CurrencyCode string      `json:"currencyCode,omitempty"`
	DailyBudget  Decimal     `json:"dailyBudget,omitempty"`
	Timezone     string      `json:"timezone,omitempty"`
	AccountInfo  AccountInfo `json:"accountInfo,omitempty"`
}

func (profile *Profile) UnmarshalJSON(data []byte) error {
	var wire struct {
		ProfileID    json.RawMessage `json:"profileId"`
		CountryCode  string          `json:"countryCode"`
		CurrencyCode string          `json:"currencyCode"`
		DailyBudget  Decimal         `json:"dailyBudget"`
		Timezone     string          `json:"timezone"`
		AccountInfo  AccountInfo     `json:"accountInfo"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	id, err := decodeID(wire.ProfileID)
	if err != nil {
		return err
	}
	*profile = Profile{ID: id, CountryCode: wire.CountryCode, CurrencyCode: wire.CurrencyCode, DailyBudget: wire.DailyBudget, Timezone: wire.Timezone, AccountInfo: wire.AccountInfo}
	return nil
}

func decodeID(data []byte) (string, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return "", fmt.Errorf("amazonads: missing profile ID")
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		if !validID(value) {
			return "", fmt.Errorf("amazonads: invalid profile ID")
		}
		return value, nil
	}
	if !validID(string(data)) {
		return "", fmt.Errorf("amazonads: invalid profile ID")
	}
	return string(data), nil
}

type Campaign struct {
	ID             string          `json:"campaignId"`
	Name           string          `json:"name,omitempty"`
	TargetingType  TargetingType   `json:"targetingType,omitempty"`
	State          State           `json:"state,omitempty"`
	StartDate      string          `json:"startDate,omitempty"`
	EndDate        string          `json:"endDate,omitempty"`
	Budget         Budget          `json:"budget,omitempty"`
	DynamicBidding *DynamicBidding `json:"dynamicBidding,omitempty"`
	PortfolioID    string          `json:"portfolioId,omitempty"`
}

type AdGroup struct {
	ID         string  `json:"adGroupId"`
	CampaignID string  `json:"campaignId,omitempty"`
	Name       string  `json:"name,omitempty"`
	DefaultBid Decimal `json:"defaultBid,omitempty"`
	State      State   `json:"state,omitempty"`
}

type ProductAd struct {
	ID         string `json:"adId"`
	CampaignID string `json:"campaignId,omitempty"`
	AdGroupID  string `json:"adGroupId,omitempty"`
	ASIN       string `json:"asin,omitempty"`
	SKU        string `json:"sku,omitempty"`
	CustomText string `json:"customText,omitempty"`
	State      State  `json:"state,omitempty"`
}

type Keyword struct {
	ID         string    `json:"keywordId"`
	CampaignID string    `json:"campaignId,omitempty"`
	AdGroupID  string    `json:"adGroupId,omitempty"`
	Text       string    `json:"keywordText,omitempty"`
	MatchType  MatchType `json:"matchType,omitempty"`
	Bid        Decimal   `json:"bid,omitempty"`
	State      State     `json:"state,omitempty"`
}

type Page[T any] struct {
	Items        []T
	NextToken    string
	TotalResults int
}

type ListCampaignsRequest struct {
	IDs        []string
	States     []State
	MaxResults int
	NextToken  string
}

type CreateCampaignRequest struct {
	Name           string
	TargetingType  TargetingType
	StartDate      string
	EndDate        string
	DailyBudget    Decimal
	DynamicBidding DynamicBidding
	PortfolioID    string
}

type UpdateCampaignRequest struct {
	Name           *string
	EndDate        *string
	DailyBudget    *Decimal
	DynamicBidding *DynamicBidding
	PortfolioID    *string
}

type ListAdGroupsRequest struct {
	IDs         []string
	CampaignIDs []string
	States      []State
	MaxResults  int
	NextToken   string
}

type CreateAdGroupRequest struct {
	CampaignID string
	Name       string
	DefaultBid Decimal
}

type UpdateAdGroupRequest struct {
	Name       *string
	DefaultBid *Decimal
}

type ListProductAdsRequest struct {
	IDs         []string
	CampaignIDs []string
	AdGroupIDs  []string
	States      []State
	MaxResults  int
	NextToken   string
}

type CreateProductAdRequest struct {
	CampaignID string
	AdGroupID  string
	ASIN       string
	SKU        string
	CustomText string
}

type ListKeywordsRequest struct {
	IDs         []string
	CampaignIDs []string
	AdGroupIDs  []string
	States      []State
	MatchTypes  []MatchType
	MaxResults  int
	NextToken   string
}

type CreateKeywordRequest struct {
	CampaignID string
	AdGroupID  string
	Text       string
	MatchType  MatchType
	Bid        Decimal
}

type UpdateKeywordRequest struct {
	Bid *Decimal
}

type ReportTimeUnit string

const (
	ReportTimeDaily   ReportTimeUnit = "DAILY"
	ReportTimeSummary ReportTimeUnit = "SUMMARY"
)

type ReportFormat string

const ReportFormatGZIPJSON ReportFormat = "GZIP_JSON"

type ReportConfiguration struct {
	AdProduct    string         `json:"adProduct"`
	GroupBy      []string       `json:"groupBy"`
	Columns      []string       `json:"columns"`
	ReportTypeID string         `json:"reportTypeId"`
	TimeUnit     ReportTimeUnit `json:"timeUnit"`
	Format       ReportFormat   `json:"format"`
}

type CreateReportRequest struct {
	Name         string
	StartDate    string
	EndDate      string
	GroupBy      []string
	Columns      []string
	ReportTypeID string
	TimeUnit     ReportTimeUnit
	Format       ReportFormat
}

type Report struct {
	ID            string              `json:"reportId"`
	Name          string              `json:"name,omitempty"`
	Status        string              `json:"status,omitempty"`
	StatusDetails string              `json:"statusDetails,omitempty"`
	CreatedAt     string              `json:"createdAt,omitempty"`
	UpdatedAt     string              `json:"updatedAt,omitempty"`
	StartDate     string              `json:"startDate,omitempty"`
	EndDate       string              `json:"endDate,omitempty"`
	Configuration ReportConfiguration `json:"configuration,omitempty"`
	URL           string              `json:"url,omitempty"`
	URLExpiresAt  string              `json:"urlExpiresAt,omitempty"`
	FailureReason string              `json:"failureReason,omitempty"`
}

type mutationItemError struct {
	Type  string          `json:"errorType"`
	Value json.RawMessage `json:"errorValue"`
}

type mutationFailure struct {
	Index  int                 `json:"index"`
	Errors []mutationItemError `json:"errors"`
}

type ProfileWorkflow interface {
	ListProfiles(context.Context, ...socialhub.CallOption) ([]Profile, error)
	GetProfile(context.Context, ...socialhub.CallOption) (*Profile, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (Page[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignState(context.Context, string, State, ...socialhub.CallOption) (*Campaign, error)
	ArchiveCampaign(context.Context, string, ...socialhub.CallOption) error
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListAdGroupsRequest, ...socialhub.CallOption) (Page[AdGroup], error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, string, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupState(context.Context, string, State, ...socialhub.CallOption) (*AdGroup, error)
	ArchiveAdGroup(context.Context, string, ...socialhub.CallOption) error
}

type ProductAdWorkflow interface {
	ListProductAds(context.Context, ListProductAdsRequest, ...socialhub.CallOption) (Page[ProductAd], error)
	CreateProductAd(context.Context, CreateProductAdRequest, ...socialhub.CallOption) (*ProductAd, error)
	SetProductAdState(context.Context, string, State, ...socialhub.CallOption) (*ProductAd, error)
	ArchiveProductAd(context.Context, string, ...socialhub.CallOption) error
}

type KeywordWorkflow interface {
	ListKeywords(context.Context, ListKeywordsRequest, ...socialhub.CallOption) (Page[Keyword], error)
	CreateKeyword(context.Context, CreateKeywordRequest, ...socialhub.CallOption) (*Keyword, error)
	UpdateKeyword(context.Context, string, UpdateKeywordRequest, ...socialhub.CallOption) (*Keyword, error)
	SetKeywordState(context.Context, string, State, ...socialhub.CallOption) (*Keyword, error)
	ArchiveKeyword(context.Context, string, ...socialhub.CallOption) error
}

type ReportWorkflow interface {
	CreateReport(context.Context, CreateReportRequest, ...socialhub.CallOption) (*Report, error)
	GetReport(context.Context, string, ...socialhub.CallOption) (*Report, error)
}
