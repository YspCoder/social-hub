package microsoftads

import (
	"context"
	"encoding/json"
	"io"

	"social-hub/pkg/socialhub"
)

// AccountWorkflow exposes the configured Microsoft Advertising account.
type AccountWorkflow interface {
	GetAccount(context.Context, ...socialhub.CallOption) (*Account, error)
}

// CampaignWorkflow manages Search Campaigns. Creates are always Paused;
// enabling delivery is a separate explicit call.
type CampaignWorkflow interface {
	ListCampaigns(context.Context, ...socialhub.CallOption) ([]Campaign, error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignStatus(context.Context, string, Status, ...socialhub.CallOption) (*Campaign, error)
}

// AdGroupWorkflow manages Search Ad Groups scoped to a Campaign. Creates are
// always Paused; enabling delivery is a separate explicit call.
type AdGroupWorkflow interface {
	ListAdGroups(context.Context, string, ...socialhub.CallOption) ([]AdGroup, error)
	GetAdGroup(context.Context, string, string, ...socialhub.CallOption) (*AdGroup, error)
	CreateAdGroup(context.Context, string, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, string, string, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupStatus(context.Context, string, string, Status, ...socialhub.CallOption) (*AdGroup, error)
}

// AdWorkflow manages responsive search ads scoped to an Ad Group. Creates are
// always Paused; enabling delivery is a separate explicit call.
type AdWorkflow interface {
	ListResponsiveSearchAds(context.Context, string, string, ...socialhub.CallOption) ([]ResponsiveSearchAd, error)
	GetResponsiveSearchAd(context.Context, string, string, string, ...socialhub.CallOption) (*ResponsiveSearchAd, error)
	CreateResponsiveSearchAd(context.Context, string, string, CreateResponsiveSearchAdRequest, ...socialhub.CallOption) (*ResponsiveSearchAd, error)
	UpdateResponsiveSearchAd(context.Context, string, string, string, UpdateResponsiveSearchAdRequest, ...socialhub.CallOption) (*ResponsiveSearchAd, error)
	SetResponsiveSearchAdStatus(context.Context, string, string, string, Status, ...socialhub.CallOption) (*ResponsiveSearchAd, error)
}

// KeywordWorkflow manages Keywords scoped to an Ad Group. Creates are always
// Paused; enabling delivery is a separate explicit call.
type KeywordWorkflow interface {
	ListKeywords(context.Context, string, string, ...socialhub.CallOption) ([]Keyword, error)
	GetKeyword(context.Context, string, string, string, ...socialhub.CallOption) (*Keyword, error)
	CreateKeyword(context.Context, string, string, CreateKeywordRequest, ...socialhub.CallOption) (*Keyword, error)
	UpdateKeyword(context.Context, string, string, string, UpdateKeywordRequest, ...socialhub.CallOption) (*Keyword, error)
	SetKeywordStatus(context.Context, string, string, string, Status, ...socialhub.CallOption) (*Keyword, error)
}

// ReportWorkflow exposes asynchronous Campaign Performance reporting and
// bounded report downloads.
type ReportWorkflow interface {
	SubmitCampaignPerformanceReport(context.Context, CampaignPerformanceReportRequest, ...socialhub.CallOption) (string, error)
	PollReport(context.Context, string, ...socialhub.CallOption) (ReportRequestStatus, error)
	DownloadReport(context.Context, string, io.Writer, ...socialhub.CallOption) (int64, error)
}

type Status string

const (
	StatusActive Status = "Active"
	StatusPaused Status = "Paused"
)

type MatchType string

const (
	MatchTypeBroad  MatchType = "Broad"
	MatchTypeExact  MatchType = "Exact"
	MatchTypePhrase MatchType = "Phrase"
)

type Network string

const (
	NetworkOwnedAndOperatedAndSyndicatedSearch Network = "OwnedAndOperatedAndSyndicatedSearch"
	NetworkOwnedAndOperatedOnly                Network = "OwnedAndOperatedOnly"
	NetworkSyndicatedSearchOnly                Network = "SyndicatedSearchOnly"
)

type Account struct {
	ID                     string          `json:"Id"`
	Name                   string          `json:"Name,omitempty"`
	Number                 string          `json:"Number,omitempty"`
	ParentCustomerID       string          `json:"ParentCustomerId,omitempty"`
	CurrencyCode           string          `json:"CurrencyCode,omitempty"`
	TimeZone               string          `json:"TimeZone,omitempty"`
	AccountLifeCycleStatus string          `json:"AccountLifeCycleStatus,omitempty"`
	AccountFinancialStatus string          `json:"AccountFinancialStatus,omitempty"`
	Raw                    json.RawMessage `json:"-"`
}

func (value *Account) UnmarshalJSON(data []byte) error {
	type alias Account
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Account(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Campaign struct {
	ID           string          `json:"Id,omitempty"`
	Name         string          `json:"Name,omitempty"`
	Status       Status          `json:"Status,omitempty"`
	BudgetType   string          `json:"BudgetType,omitempty"`
	DailyBudget  float64         `json:"DailyBudget,omitempty"`
	TimeZone     string          `json:"TimeZone,omitempty"`
	CampaignType string          `json:"CampaignType,omitempty"`
	Languages    []string        `json:"Languages,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

func (value *Campaign) UnmarshalJSON(data []byte) error {
	type alias Campaign
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Campaign(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateCampaignRequest struct {
	Name        string
	DailyBudget float64
	TimeZone    string
	Languages   []string
}

type UpdateCampaignRequest struct {
	Name        *string
	DailyBudget *float64
	TimeZone    *string
	Languages   *[]string
}

type Bid struct {
	Amount float64 `json:"Amount"`
}

type AdGroup struct {
	ID       string          `json:"Id,omitempty"`
	Name     string          `json:"Name,omitempty"`
	Status   Status          `json:"Status,omitempty"`
	CPCBid   *Bid            `json:"CpcBid,omitempty"`
	Language string          `json:"Language,omitempty"`
	Network  Network         `json:"Network,omitempty"`
	Raw      json.RawMessage `json:"-"`
}

func (value *AdGroup) UnmarshalJSON(data []byte) error {
	type alias AdGroup
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AdGroup(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateAdGroupRequest struct {
	Name     string
	CPCBid   *float64
	Language string
	Network  Network
}

type UpdateAdGroupRequest struct {
	Name     *string
	CPCBid   *float64
	Language *string
	Network  *Network
}

type TextAsset struct {
	Type string `json:"Type,omitempty"`
	Text string `json:"Text"`
}

type AssetLink struct {
	Asset                 TextAsset `json:"Asset"`
	PinnedField           string    `json:"PinnedField,omitempty"`
	AssetPerformanceLabel string    `json:"AssetPerformanceLabel,omitempty"`
}

type AdTextAsset struct {
	Text        string
	PinnedField string
}

type ResponsiveSearchAd struct {
	ID              string          `json:"Id,omitempty"`
	Type            string          `json:"Type,omitempty"`
	Status          Status          `json:"Status,omitempty"`
	EditorialStatus string          `json:"EditorialStatus,omitempty"`
	FinalURLs       []string        `json:"FinalUrls,omitempty"`
	Headlines       []AssetLink     `json:"Headlines,omitempty"`
	Descriptions    []AssetLink     `json:"Descriptions,omitempty"`
	Path1           string          `json:"Path1,omitempty"`
	Path2           string          `json:"Path2,omitempty"`
	Raw             json.RawMessage `json:"-"`
}

func (value *ResponsiveSearchAd) UnmarshalJSON(data []byte) error {
	type alias ResponsiveSearchAd
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = ResponsiveSearchAd(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateResponsiveSearchAdRequest struct {
	FinalURLs    []string
	Headlines    []AdTextAsset
	Descriptions []AdTextAsset
	Path1        string
	Path2        string
}

type UpdateResponsiveSearchAdRequest struct {
	FinalURLs    *[]string
	Headlines    *[]AdTextAsset
	Descriptions *[]AdTextAsset
	Path1        *string
	Path2        *string
}

type Keyword struct {
	ID              string          `json:"Id,omitempty"`
	Text            string          `json:"Text,omitempty"`
	Status          Status          `json:"Status,omitempty"`
	MatchType       MatchType       `json:"MatchType,omitempty"`
	Bid             *Bid            `json:"Bid,omitempty"`
	FinalURLs       []string        `json:"FinalUrls,omitempty"`
	EditorialStatus string          `json:"EditorialStatus,omitempty"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Keyword) UnmarshalJSON(data []byte) error {
	type alias Keyword
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Keyword(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateKeywordRequest struct {
	Text      string
	MatchType MatchType
	Bid       *float64
	FinalURLs []string
}

type UpdateKeywordRequest struct {
	Text      *string
	MatchType *MatchType
	Bid       *float64
	FinalURLs *[]string
}

type ReportFormat string

const (
	ReportFormatCSV ReportFormat = "Csv"
	ReportFormatTSV ReportFormat = "Tsv"
)

type ReportDate struct {
	Day   int `json:"Day"`
	Month int `json:"Month"`
	Year  int `json:"Year"`
}

type ReportTime struct {
	CustomDateRangeEnd   *ReportDate `json:"CustomDateRangeEnd,omitempty"`
	CustomDateRangeStart *ReportDate `json:"CustomDateRangeStart,omitempty"`
	PredefinedTime       string      `json:"PredefinedTime,omitempty"`
	ReportTimeZone       string      `json:"ReportTimeZone,omitempty"`
}

type CampaignPerformanceReportRequest struct {
	ReportName             string
	Format                 ReportFormat
	Aggregation            string
	Columns                []string
	Time                   ReportTime
	CampaignIDs            []string
	ExcludeColumnHeaders   bool
	ExcludeReportFooter    bool
	ExcludeReportHeader    bool
	ReturnOnlyCompleteData bool
}

type ReportRequestStatus struct {
	Status            string `json:"Status"`
	ReportDownloadURL string `json:"ReportDownloadUrl,omitempty"`
}
