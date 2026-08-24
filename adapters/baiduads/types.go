package baiduads

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type Account struct {
	UserID     int64           `json:"userId"`
	Balance    float64         `json:"balance,omitempty"`
	PCBalance  float64         `json:"pcBalance,omitempty"`
	Cost       float64         `json:"cost,omitempty"`
	Payment    float64         `json:"payment,omitempty"`
	Budget     float64         `json:"budget,omitempty"`
	BudgetType int             `json:"budgetType,omitempty"`
	RegDomain  string          `json:"regDomain,omitempty"`
	UserStatus int             `json:"userStat,omitempty"`
	UserLevel  int             `json:"userLevel,omitempty"`
	Raw        json.RawMessage `json:"-"`
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
	ID                int64           `json:"campaignId"`
	Name              string          `json:"campaignName,omitempty"`
	Budget            float64         `json:"budget,omitempty"`
	Pause             bool            `json:"pause,omitempty"`
	Status            int             `json:"status,omitempty"`
	AdType            int             `json:"adType,omitempty"`
	MarketingTargetID int             `json:"marketingTargetId,omitempty"`
	BusinessPointID   int64           `json:"businessPointId,omitempty"`
	EquipmentType     int             `json:"equipmentType,omitempty"`
	CreatedTime       string          `json:"createTime,omitempty"`
	Raw               json.RawMessage `json:"-"`
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

type AdGroup struct {
	ID             int64           `json:"adgroupId"`
	CampaignID     int64           `json:"campaignId,omitempty"`
	Name           string          `json:"adgroupName,omitempty"`
	MaxPrice       float64         `json:"maxPrice,omitempty"`
	Pause          bool            `json:"pause,omitempty"`
	Status         int             `json:"status,omitempty"`
	AdType         int             `json:"adType,omitempty"`
	ProductSetID   int64           `json:"productSetId,omitempty"`
	PAProductPrice float64         `json:"paPrice,omitempty"`
	CreatedTime    string          `json:"createTime,omitempty"`
	Raw            json.RawMessage `json:"-"`
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

type OfflineReason struct {
	MainReason   string `json:"mainReason,omitempty"`
	DetailReason string `json:"detailReason,omitempty"`
}

type Creative struct {
	ID                   int64           `json:"creativeId"`
	CampaignID           int64           `json:"campaignId,omitempty"`
	AdGroupID            int64           `json:"adgroupId,omitempty"`
	Title                string          `json:"title,omitempty"`
	Description1         string          `json:"description1,omitempty"`
	Description2         string          `json:"description2,omitempty"`
	Pause                bool            `json:"pause,omitempty"`
	Status               int             `json:"status,omitempty"`
	MobileDestinationURL string          `json:"mobileDestinationUrl,omitempty"`
	MobileDisplayURL     string          `json:"mobileDisplayUrl,omitempty"`
	PCDestinationURL     string          `json:"pcDestinationUrl,omitempty"`
	PCDisplayURL         string          `json:"pcDisplayUrl,omitempty"`
	Tabs                 []int           `json:"tabs,omitempty"`
	DeepLink             string          `json:"deeplink,omitempty"`
	MiniProgramURL       string          `json:"miniProgramUrl,omitempty"`
	OfflineReasons       []OfflineReason `json:"offlineReasons,omitempty"`
	CreatedTime          string          `json:"createTime,omitempty"`
	Raw                  json.RawMessage `json:"-"`
}

func (value *Creative) UnmarshalJSON(data []byte) error {
	type alias Creative
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Creative(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type GetCampaignsRequest struct {
	IDs    []int64
	Fields []string
	AdType *int
}

type CreateCampaignRequest struct {
	Name              string
	Budget            float64
	AdType            int
	MarketingTargetID int
	Fields            map[string]any
}

type UpdateCampaignRequest struct {
	Name   *string
	Budget *float64
	Pause  *bool
	Fields map[string]any
}

type AdGroupIDType int

const (
	AdGroupByCampaignID AdGroupIDType = 3
	AdGroupByID         AdGroupIDType = 5
)

type GetAdGroupsRequest struct {
	IDs     []int64
	Fields  []string
	IDType  AdGroupIDType
	GetTemp bool
}

type CreateAdGroupRequest struct {
	CampaignID int64
	Name       string
	MaxPrice   float64
	AdType     int
	Fields     map[string]any
}

type UpdateAdGroupRequest struct {
	Name     *string
	MaxPrice *float64
	Pause    *bool
	Fields   map[string]any
}

type CreativeIDType int

const (
	CreativeByAdGroupID CreativeIDType = 5
	CreativeByID        CreativeIDType = 7
)

type GetCreativesRequest struct {
	IDs     []int64
	Fields  []string
	IDType  CreativeIDType
	GetTemp bool
}

type CreateCreativeRequest struct {
	CampaignID           int64
	AdGroupID            int64
	Title                string
	Description1         string
	Description2         string
	MobileDestinationURL string
	MobileDisplayURL     string
	PCDestinationURL     string
	PCDisplayURL         string
	Tabs                 []int
	Fields               map[string]any
}

// UpdateCreativeRequest is intentionally a full replacement shape because
// Baidu requires all core text and destination fields on every update.
type UpdateCreativeRequest struct {
	Title                string
	Description1         string
	Description2         string
	Pause                bool
	MobileDestinationURL string
	MobileDisplayURL     string
	PCDestinationURL     string
	PCDisplayURL         string
	Tabs                 []int
	Fields               map[string]any
}

type ReportTimeUnit string

const (
	ReportTimeHour    ReportTimeUnit = "HOUR"
	ReportTimeDay     ReportTimeUnit = "DAY"
	ReportTimeWeek    ReportTimeUnit = "WEEK"
	ReportTimeMonth   ReportTimeUnit = "MONTH"
	ReportTimeSummary ReportTimeUnit = "SUMMARY"
)

type ReportSort struct {
	Column string `json:"column"`
	Rule   string `json:"sortRule"`
}

type ReportFilter struct {
	Column   string   `json:"column"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

type ReportRequest struct {
	ReportType int64
	UserIDs    []int64
	StartDate  string
	EndDate    string
	TimeUnit   ReportTimeUnit
	Columns    []string
	Sorts      []ReportSort
	Filters    []ReportFilter
	StartRow   int
	RowCount   int
	NeedSum    bool
}

type ReportRow map[string]json.RawMessage

type ReportData struct {
	Rows          []ReportRow                `json:"rows"`
	Summary       map[string]json.RawMessage `json:"summary,omitempty"`
	RowCount      int                        `json:"rowCount"`
	TotalRowCount int                        `json:"totalRowCount"`
	ColumnMetas   json.RawMessage            `json:"columnMetas,omitempty"`
}

type ReportTaskError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type ReportTask struct {
	TaskID       string            `json:"taskId"`
	Status       string            `json:"taskStatus,omitempty"`
	SubmitTime   string            `json:"submitTime,omitempty"`
	StartTime    string            `json:"startTime,omitempty"`
	FinishTime   string            `json:"finishTime,omitempty"`
	FileURL      string            `json:"fileUrl,omitempty"`
	Errors       []ReportTaskError `json:"errors,omitempty"`
	DataStartRow int               `json:"dataStartRow,omitempty"`
	TableHeader  []string          `json:"tableHeader,omitempty"`
}

type AuthorizationRequest struct {
	Callback string
	Scope    string
	State    string
}

type OAuthToken struct {
	Token              socialhub.Token
	OpenID             string
	UserID             int64
	Scope              map[string]string
	ExpiresTime        string
	RefreshExpiresAt   time.Time
	RefreshExpiresTime string
}

type AccountWorkflow interface {
	GetAccount(context.Context, []string, ...socialhub.CallOption) (*Account, error)
}

type CampaignWorkflow interface {
	GetCampaign(context.Context, int64, []string, ...socialhub.CallOption) (*Campaign, error)
	GetCampaigns(context.Context, GetCampaignsRequest, ...socialhub.CallOption) ([]Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, int64, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	DeleteCampaign(context.Context, int64, ...socialhub.CallOption) error
}

type AdGroupWorkflow interface {
	GetAdGroup(context.Context, int64, []string, ...socialhub.CallOption) (*AdGroup, error)
	GetAdGroups(context.Context, GetAdGroupsRequest, ...socialhub.CallOption) ([]AdGroup, error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, int64, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	DeleteAdGroup(context.Context, int64, ...socialhub.CallOption) error
}

type CreativeWorkflow interface {
	GetCreative(context.Context, int64, []string, ...socialhub.CallOption) (*Creative, error)
	GetCreatives(context.Context, GetCreativesRequest, ...socialhub.CallOption) ([]Creative, error)
	CreateCreative(context.Context, CreateCreativeRequest, ...socialhub.CallOption) (*Creative, error)
	UpdateCreative(context.Context, int64, UpdateCreativeRequest, ...socialhub.CallOption) (*Creative, error)
	DeleteCreative(context.Context, int64, ...socialhub.CallOption) error
}

type ReportWorkflow interface {
	GetReportData(context.Context, ReportRequest, ...socialhub.CallOption) (*ReportData, error)
	CreateReportTask(context.Context, ReportRequest, ...socialhub.CallOption) (*ReportTask, error)
	GetReportTask(context.Context, string, ...socialhub.CallOption) (*ReportTask, error)
}
