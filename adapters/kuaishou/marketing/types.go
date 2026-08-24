package marketing

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type PutStatus int

const (
	PutStatusDelivering PutStatus = 1
	PutStatusPaused     PutStatus = 2
	PutStatusDeleted    PutStatus = 3
)

type BidType int

const (
	BidTypeCPC  BidType = 2
	BidTypeOCPM BidType = 10
	BidTypeMCB  BidType = 12
)

type NumberPage[T any] struct {
	Items       []T
	Page        int
	PageSize    int
	TotalNumber int64
	HasMore     bool
}

type Advertiser struct {
	AdvertiserID      int64           `json:"advertiser_id,omitempty"`
	UserID            int64           `json:"user_id,omitempty"`
	UserName          string          `json:"user_name,omitempty"`
	CorporationName   string          `json:"corporation_name,omitempty"`
	PrimaryIndustryID int64           `json:"primary_industry_id,omitempty"`
	PrimaryIndustry   string          `json:"primary_industry_name,omitempty"`
	IndustryID        int64           `json:"industry_id,omitempty"`
	IndustryName      string          `json:"industry_name,omitempty"`
	ProductName       string          `json:"product_name,omitempty"`
	DeliveryType      int             `json:"delivery_type,omitempty"`
	EffectFirst       int             `json:"effect_first,omitempty"`
	Raw               json.RawMessage `json:"-"`
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
	ID                int64           `json:"campaign_id"`
	AdvertiserID      int64           `json:"advertiser_id,omitempty"`
	Name              string          `json:"campaign_name,omitempty"`
	PutStatus         PutStatus       `json:"put_status,omitempty"`
	Status            int             `json:"status,omitempty"`
	DayBudget         int64           `json:"day_budget,omitempty"`
	DayBudgetSchedule []int64         `json:"day_budget_schedule,omitempty"`
	MarketingGoal     int             `json:"campaign_type,omitempty"`
	AdType            int             `json:"ad_type,omitempty"`
	BidType           int             `json:"bid_type,omitempty"`
	CreatedTime       string          `json:"create_time,omitempty"`
	UpdatedTime       string          `json:"update_time,omitempty"`
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

type CreateCampaignRequest struct {
	Name              string
	MarketingGoal     int
	AdType            int
	BidType           int
	DayBudget         int64
	DayBudgetSchedule []int64
	Fields            map[string]any
}

type UpdateCampaignRequest struct {
	Name              *string
	DayBudget         *int64
	DayBudgetSchedule *[]int64
	Fields            map[string]any
}

type ListCampaignsRequest struct {
	IDs            []int64
	Name           string
	PutStatuses    []PutStatus
	Status         *int
	StartDate      string
	EndDate        string
	TimeFilterType int
	Page           int
	PageSize       int
}

type Unit struct {
	ID                int64           `json:"unit_id"`
	AdvertiserID      int64           `json:"advertiser_id,omitempty"`
	CampaignID        int64           `json:"campaign_id,omitempty"`
	Name              string          `json:"unit_name,omitempty"`
	PutStatus         PutStatus       `json:"put_status,omitempty"`
	Status            int             `json:"status,omitempty"`
	ReviewStatus      int             `json:"review_status,omitempty"`
	BidType           BidType         `json:"bid_type,omitempty"`
	Bid               int64           `json:"bid,omitempty"`
	CPABid            int64           `json:"cpa_bid,omitempty"`
	DayBudget         int64           `json:"day_budget,omitempty"`
	DayBudgetSchedule []int64         `json:"day_budget_schedule,omitempty"`
	BeginTime         string          `json:"begin_time,omitempty"`
	EndTime           string          `json:"end_time,omitempty"`
	SceneIDs          []string        `json:"scene_id,omitempty"`
	UnitType          int             `json:"unit_type,omitempty"`
	CreatedTime       string          `json:"create_time,omitempty"`
	UpdatedTime       string          `json:"update_time,omitempty"`
	Target            json.RawMessage `json:"target,omitempty"`
	Raw               json.RawMessage `json:"-"`
}

func (value *Unit) UnmarshalJSON(data []byte) error {
	type alias Unit
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Unit(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateUnitRequest struct {
	CampaignID        int64
	Name              string
	BeginTime         string
	EndTime           string
	BidType           BidType
	Bid               int64
	CPABid            int64
	SceneIDs          []string
	UnitType          int
	Target            map[string]any
	DayBudget         int64
	DayBudgetSchedule []int64
	Fields            map[string]any
}

type ListUnitsRequest struct {
	IDs            []int64
	CampaignID     int64
	Name           string
	PutStatuses    []PutStatus
	StartDate      string
	EndDate        string
	TimeFilterType int
	Page           int
	PageSize       int
}

type Creative struct {
	ID            int64           `json:"creative_id"`
	AdvertiserID  int64           `json:"advertiser_id,omitempty"`
	CampaignID    int64           `json:"campaign_id,omitempty"`
	UnitID        int64           `json:"unit_id,omitempty"`
	Name          string          `json:"creative_name,omitempty"`
	PutStatus     PutStatus       `json:"put_status,omitempty"`
	Status        int             `json:"status,omitempty"`
	MaterialType  int             `json:"creative_material_type,omitempty"`
	ActionBarText string          `json:"action_bar_text,omitempty"`
	Description   string          `json:"description,omitempty"`
	PhotoID       string          `json:"photo_id,omitempty"`
	ImageToken    string          `json:"image_token,omitempty"`
	ImageTokens   []string        `json:"image_tokens,omitempty"`
	MaterialURLs  []string        `json:"material_url,omitempty"`
	CreatedTime   string          `json:"create_time,omitempty"`
	UpdatedTime   string          `json:"update_time,omitempty"`
	Raw           json.RawMessage `json:"-"`
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

type CreateCreativeRequest struct {
	UnitID        int64
	Name          string
	MaterialType  int
	ActionBarText string
	Description   string
	PhotoID       string
	ImageToken    string
	ImageTokens   []string
	Fields        map[string]any
}

type ListCreativesRequest struct {
	IDs            []int64
	CampaignID     int64
	UnitID         int64
	Name           string
	PutStatuses    []PutStatus
	StartDate      string
	EndDate        string
	TimeFilterType int
	Page           int
	PageSize       int
}

type BatchError struct {
	ID      int64  `json:"id,omitempty"`
	Code    int64  `json:"error_code,omitempty"`
	Message string `json:"error_msg,omitempty"`
}

type BatchResult struct {
	SucceededIDs []int64
	Errors       []BatchError
}

type ReportLevel string

const (
	ReportLevelAccount  ReportLevel = "ACCOUNT"
	ReportLevelCampaign ReportLevel = "CAMPAIGN"
	ReportLevelUnit     ReportLevel = "UNIT"
	ReportLevelCreative ReportLevel = "CREATIVE"
)

type TemporalGranularity string

const (
	GranularityDaily  TemporalGranularity = "DAILY"
	GranularityHourly TemporalGranularity = "HOURLY"
)

type ReportRequest struct {
	Level               ReportLevel
	StartDate           string
	EndDate             string
	TemporalGranularity TemporalGranularity
	ReportDimensions    []string
	CampaignType        int
	CampaignIDs         []int64
	UnitIDs             []int64
	CreativeIDs         []int64
	ExtendInfo          []string
	Page                int
	PageSize            int
}

type ReportRow struct {
	AdvertiserID int64           `json:"advertiser_id,omitempty"`
	AccountID    int64           `json:"account_id,omitempty"`
	CampaignID   int64           `json:"campaign_id,omitempty"`
	UnitID       int64           `json:"unit_id,omitempty"`
	CreativeID   int64           `json:"creative_id,omitempty"`
	Date         string          `json:"stat_date,omitempty"`
	Hour         int             `json:"stat_hour,omitempty"`
	Raw          json.RawMessage `json:"-"`
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

type OAuthToken struct {
	Token            socialhub.Token
	RefreshExpiresAt time.Time
}

type AuthorizationRequest struct {
	RedirectURI string
	State       string
	Scopes      []string
	OAuthType   string
}

type AdvertiserWorkflow interface {
	GetAdvertiser(context.Context, ...socialhub.CallOption) (*Advertiser, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (NumberPage[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, int64, UpdateCampaignRequest, ...socialhub.CallOption) error
	SetCampaignStatus(context.Context, int64, PutStatus, ...socialhub.CallOption) (BatchResult, error)
}

type UnitWorkflow interface {
	ListUnits(context.Context, ListUnitsRequest, ...socialhub.CallOption) (NumberPage[Unit], error)
	CreateUnit(context.Context, CreateUnitRequest, ...socialhub.CallOption) (*Unit, error)
	SetUnitStatus(context.Context, int64, PutStatus, ...socialhub.CallOption) (BatchResult, error)
}

type CreativeWorkflow interface {
	ListCreatives(context.Context, ListCreativesRequest, ...socialhub.CallOption) (NumberPage[Creative], error)
	CreateCreative(context.Context, CreateCreativeRequest, ...socialhub.CallOption) (*Creative, error)
	SetCreativeStatus(context.Context, int64, PutStatus, ...socialhub.CallOption) (BatchResult, error)
}

type ReportWorkflow interface {
	GetReport(context.Context, ReportRequest, ...socialhub.CallOption) (NumberPage[ReportRow], error)
}
