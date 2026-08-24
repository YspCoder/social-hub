package tencentads

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type ConfiguredStatus string

const (
	ConfiguredStatusNormal  ConfiguredStatus = "AD_STATUS_NORMAL"
	ConfiguredStatusSuspend ConfiguredStatus = "AD_STATUS_SUSPEND"
)

type CampaignType string

const (
	CampaignTypeNormal         CampaignType = "CAMPAIGN_TYPE_NORMAL"
	CampaignTypeSearch         CampaignType = "CAMPAIGN_TYPE_SEARCH"
	CampaignTypeWechatMoments  CampaignType = "CAMPAIGN_TYPE_WECHAT_MOMENTS"
	CampaignTypeWechatOfficial CampaignType = "CAMPAIGN_TYPE_WECHAT_OFFICIAL_ACCOUNTS"
)

type PromotedObjectType string

const (
	PromotedObjectLink           PromotedObjectType = "PROMOTED_OBJECT_TYPE_LINK"
	PromotedObjectWechatLink     PromotedObjectType = "PROMOTED_OBJECT_TYPE_LINK_WECHAT"
	PromotedObjectWechatMiniGame PromotedObjectType = "PROMOTED_OBJECT_TYPE_MINI_GAME_WECHAT"
	PromotedObjectWechatMiniApp  PromotedObjectType = "PROMOTED_OBJECT_TYPE_MINI_PROGRAM_WECHAT"
	PromotedObjectWechatChannels PromotedObjectType = "PROMOTED_OBJECT_TYPE_WECHAT_CHANNELS"
	PromotedObjectWechatOfficial PromotedObjectType = "PROMOTED_OBJECT_TYPE_WECHAT_OFFICIAL_ACCOUNT"
)

type BillingEvent string

const (
	BillingEventClick      BillingEvent = "BILLINGEVENT_CLICK"
	BillingEventImpression BillingEvent = "BILLINGEVENT_IMPRESSION"
)

type OptimizationGoal string

const (
	OptimizationGoalClick      OptimizationGoal = "OPTIMIZATIONGOAL_CLICK"
	OptimizationGoalImpression OptimizationGoal = "OPTIMIZATIONGOAL_IMPRESSION"
)

type Filtering struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

type OrderBy struct {
	SortField string `json:"sort_field"`
	SortType  string `json:"sort_type"`
}

type NumberPage[T any] struct {
	Items       []T
	Page        int
	PageSize    int
	TotalNumber int64
	TotalPages  int
	HasMore     bool
}

type Advertiser struct {
	AccountID       int64           `json:"account_id"`
	DailyBudget     int64           `json:"daily_budget,omitempty"`
	SystemStatus    string          `json:"system_status,omitempty"`
	CorporationName string          `json:"corporation_name,omitempty"`
	CorporateBrand  string          `json:"corporate_brand_name,omitempty"`
	IndustryID      int64           `json:"system_industry_id,omitempty"`
	AgencyAccountID int64           `json:"agency_account_id,omitempty"`
	AreaCode        int64           `json:"area_code,omitempty"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Advertiser) UnmarshalJSON(data []byte) error {
	type alias Advertiser
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Advertiser(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Campaign struct {
	ID                 int64              `json:"campaign_id"`
	AccountID          int64              `json:"account_id,omitempty"`
	Name               string             `json:"campaign_name,omitempty"`
	ConfiguredStatus   ConfiguredStatus   `json:"configured_status,omitempty"`
	CampaignType       CampaignType       `json:"campaign_type,omitempty"`
	PromotedObjectType PromotedObjectType `json:"promoted_object_type,omitempty"`
	DailyBudget        int64              `json:"daily_budget,omitempty"`
	TotalBudget        int64              `json:"total_budget,omitempty"`
	CreatedTime        int64              `json:"created_time,omitempty"`
	LastModifiedTime   int64              `json:"last_modified_time,omitempty"`
	Deleted            bool               `json:"is_deleted,omitempty"`
	Raw                json.RawMessage    `json:"-"`
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
	Name               string
	CampaignType       CampaignType
	PromotedObjectType PromotedObjectType
	DailyBudget        int64
	TotalBudget        int64
	Fields             map[string]any
}

type UpdateCampaignRequest struct {
	Name        *string
	DailyBudget *int64
	TotalBudget *int64
	Fields      map[string]any
}

type ListCampaignsRequest struct {
	Fields         []string
	Filtering      []Filtering
	Page           int
	PageSize       int
	IncludeDeleted bool
}

type AdGroup struct {
	ID                 int64              `json:"adgroup_id"`
	AccountID          int64              `json:"account_id,omitempty"`
	CampaignID         int64              `json:"campaign_id,omitempty"`
	Name               string             `json:"adgroup_name,omitempty"`
	ConfiguredStatus   ConfiguredStatus   `json:"configured_status,omitempty"`
	SystemStatus       string             `json:"system_status,omitempty"`
	Status             string             `json:"status,omitempty"`
	PromotedObjectType PromotedObjectType `json:"promoted_object_type,omitempty"`
	PromotedObjectID   string             `json:"promoted_object_id,omitempty"`
	BillingEvent       BillingEvent       `json:"billing_event,omitempty"`
	OptimizationGoal   OptimizationGoal   `json:"optimization_goal,omitempty"`
	BidAmount          int64              `json:"bid_amount,omitempty"`
	DailyBudget        int64              `json:"daily_budget,omitempty"`
	BeginDate          string             `json:"begin_date,omitempty"`
	EndDate            string             `json:"end_date,omitempty"`
	CreatedTime        int64              `json:"created_time,omitempty"`
	LastModifiedTime   int64              `json:"last_modified_time,omitempty"`
	Deleted            bool               `json:"is_deleted,omitempty"`
	Raw                json.RawMessage    `json:"-"`
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
	CampaignID         int64
	Name               string
	PromotedObjectType PromotedObjectType
	BillingEvent       BillingEvent
	OptimizationGoal   OptimizationGoal
	BidAmount          int64
	BeginDate          string
	EndDate            string
	Fields             map[string]any
}

type UpdateAdGroupRequest struct {
	Name        *string
	BidAmount   *int64
	DailyBudget *int64
	EndDate     *string
	Fields      map[string]any
}

type ListAdGroupsRequest struct {
	Fields         []string
	Filtering      []Filtering
	Page           int
	PageSize       int
	IncludeDeleted bool
}

type AdCreative struct {
	ID                 int64              `json:"adcreative_id"`
	AccountID          int64              `json:"account_id,omitempty"`
	CampaignID         int64              `json:"campaign_id,omitempty"`
	Name               string             `json:"adcreative_name,omitempty"`
	PromotedObjectType PromotedObjectType `json:"promoted_object_type,omitempty"`
	PromotedObjectID   string             `json:"promoted_object_id,omitempty"`
	TemplateID         int64              `json:"adcreative_template_id,omitempty"`
	PageType           string             `json:"page_type,omitempty"`
	DeepLinkURL        string             `json:"deep_link_url,omitempty"`
	CreatedTime        int64              `json:"created_time,omitempty"`
	LastModifiedTime   int64              `json:"last_modified_time,omitempty"`
	Deleted            bool               `json:"is_deleted,omitempty"`
	Raw                json.RawMessage    `json:"-"`
}

func (value *AdCreative) UnmarshalJSON(data []byte) error {
	type alias AdCreative
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AdCreative(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateAdCreativeRequest struct {
	CampaignID         int64
	Name               string
	PromotedObjectType PromotedObjectType
	TemplateID         int64
	Fields             map[string]any
}

type UpdateAdCreativeRequest struct {
	Name   *string
	Fields map[string]any
}

type ListAdCreativesRequest struct {
	Fields         []string
	Filtering      []Filtering
	Page           int
	PageSize       int
	IncludeDeleted bool
}

type ReportGranularity string

const (
	ReportDaily  ReportGranularity = "daily"
	ReportHourly ReportGranularity = "hourly"
)

type ReportLevel string

const (
	ReportLevelAdvertiser ReportLevel = "REPORT_LEVEL_ADVERTISER"
	ReportLevelCampaign   ReportLevel = "REPORT_LEVEL_CAMPAIGN"
	ReportLevelAdGroup    ReportLevel = "REPORT_LEVEL_ADGROUP"
	ReportLevelAd         ReportLevel = "REPORT_LEVEL_AD"
)

type ReportDateRange struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type ReportRequest struct {
	Granularity ReportGranularity
	Level       ReportLevel
	DateRange   ReportDateRange
	Fields      []string
	Filtering   []Filtering
	GroupBy     []string
	OrderBy     []OrderBy
	TimeLine    string
	Page        int
	PageSize    int
}

type ReportRow struct {
	AccountID  int64           `json:"account_id,omitempty"`
	CampaignID int64           `json:"campaign_id,omitempty"`
	AdGroupID  int64           `json:"adgroup_id,omitempty"`
	AdID       int64           `json:"ad_id,omitempty"`
	Date       string          `json:"date,omitempty"`
	Hour       int64           `json:"hour,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

func (value *ReportRow) UnmarshalJSON(data []byte) error {
	type alias ReportRow
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = ReportRow(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type AuthorizerInfo struct {
	AccountUIN      int64    `json:"account_uin,omitempty"`
	AccountID       int64    `json:"account_id,omitempty"`
	Scopes          []string `json:"scope_list,omitempty"`
	WechatAccountID string   `json:"wechat_account_id,omitempty"`
	AccountRoleType string   `json:"account_role_type,omitempty"`
	AccountType     string   `json:"account_type,omitempty"`
	RoleType        string   `json:"role_type,omitempty"`
}

type OAuthToken struct {
	Token            socialhub.Token
	Authorizer       AuthorizerInfo
	RefreshExpiresAt time.Time
}

type AuthorizationRequest struct {
	RedirectURI          string
	State                string
	Scope                string
	AccountType          string
	AccountDisplayNumber int64
	Fields               []string
}

type AdvertiserWorkflow interface {
	GetAdvertiser(context.Context, ...socialhub.CallOption) (*Advertiser, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (NumberPage[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, int64, UpdateCampaignRequest, ...socialhub.CallOption) error
	SetCampaignStatus(context.Context, int64, ConfiguredStatus, ...socialhub.CallOption) error
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListAdGroupsRequest, ...socialhub.CallOption) (NumberPage[AdGroup], error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, int64, UpdateAdGroupRequest, ...socialhub.CallOption) error
	SetAdGroupStatus(context.Context, int64, ConfiguredStatus, ...socialhub.CallOption) error
}

type AdCreativeWorkflow interface {
	ListAdCreatives(context.Context, ListAdCreativesRequest, ...socialhub.CallOption) (NumberPage[AdCreative], error)
	CreateAdCreative(context.Context, CreateAdCreativeRequest, ...socialhub.CallOption) (*AdCreative, error)
	UpdateAdCreative(context.Context, int64, UpdateAdCreativeRequest, ...socialhub.CallOption) error
}

type ReportWorkflow interface {
	GetReport(context.Context, ReportRequest, ...socialhub.CallOption) (NumberPage[ReportRow], error)
}
