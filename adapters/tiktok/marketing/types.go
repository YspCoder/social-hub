package marketing

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type OperationStatus string

const (
	StatusEnable  OperationStatus = "ENABLE"
	StatusDisable OperationStatus = "DISABLE"
	StatusDelete  OperationStatus = "DELETE"
)

type NumberPage[T any] struct {
	Items       []T
	Page        int
	PageSize    int
	TotalNumber int64
	TotalPage   int
	HasMore     bool
}

type pageInfo struct {
	Page        int   `json:"page"`
	PageSize    int   `json:"page_size"`
	TotalNumber int64 `json:"total_number"`
	TotalPage   int   `json:"total_page"`
	HasMore     bool  `json:"has_more"`
}

type Advertiser struct {
	ID          string          `json:"advertiser_id"`
	Name        string          `json:"name,omitempty"`
	Company     string          `json:"company,omitempty"`
	Status      string          `json:"status,omitempty"`
	Role        string          `json:"role,omitempty"`
	Country     string          `json:"country,omitempty"`
	Currency    string          `json:"currency,omitempty"`
	Industry    string          `json:"industry,omitempty"`
	Timezone    json.RawMessage `json:"timezone,omitempty"`
	OwnerBCID   string          `json:"owner_bc_id,omitempty"`
	CreatedTime int64           `json:"create_time,omitempty"`
	Raw         json.RawMessage `json:"-"`
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
	ID                string          `json:"campaign_id"`
	AdvertiserID      string          `json:"advertiser_id,omitempty"`
	Name              string          `json:"campaign_name,omitempty"`
	ObjectiveType     string          `json:"objective_type,omitempty"`
	CampaignType      string          `json:"campaign_type,omitempty"`
	BudgetMode        string          `json:"budget_mode,omitempty"`
	Budget            float64         `json:"budget,omitempty"`
	BudgetOptimizeOn  bool            `json:"budget_optimize_on,omitempty"`
	OperationStatus   OperationStatus `json:"operation_status,omitempty"`
	SecondaryStatus   string          `json:"secondary_status,omitempty"`
	CreateTime        string          `json:"create_time,omitempty"`
	ModifyTime        string          `json:"modify_time,omitempty"`
	SpecialIndustries []string        `json:"special_industries,omitempty"`
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
	ObjectiveType     string
	CampaignType      string
	BudgetMode        string
	Budget            float64
	BudgetOptimizeOn  *bool
	SpecialIndustries []string
	RequestID         string
	Fields            map[string]any
}

type UpdateCampaignRequest struct {
	Name   *string
	Budget *float64
	Fields map[string]any
}

type ListCampaignsRequest struct {
	IDs             []string
	Name            string
	ObjectiveType   string
	PrimaryStatus   string
	SecondaryStatus string
	Fields          []string
	Page            int
	PageSize        int
}

type AdGroup struct {
	ID               string          `json:"adgroup_id"`
	AdvertiserID     string          `json:"advertiser_id,omitempty"`
	CampaignID       string          `json:"campaign_id,omitempty"`
	CampaignName     string          `json:"campaign_name,omitempty"`
	Name             string          `json:"adgroup_name,omitempty"`
	PromotionType    string          `json:"promotion_type,omitempty"`
	PlacementType    string          `json:"placement_type,omitempty"`
	Placements       []string        `json:"placements,omitempty"`
	BudgetMode       string          `json:"budget_mode,omitempty"`
	Budget           float64         `json:"budget,omitempty"`
	OptimizationGoal string          `json:"optimization_goal,omitempty"`
	BillingEvent     string          `json:"billing_event,omitempty"`
	BidType          string          `json:"bid_type,omitempty"`
	OperationStatus  OperationStatus `json:"operation_status,omitempty"`
	SecondaryStatus  string          `json:"secondary_status,omitempty"`
	ScheduleType     string          `json:"schedule_type,omitempty"`
	ScheduleStart    string          `json:"schedule_start_time,omitempty"`
	ScheduleEnd      string          `json:"schedule_end_time,omitempty"`
	CreateTime       string          `json:"create_time,omitempty"`
	ModifyTime       string          `json:"modify_time,omitempty"`
	Raw              json.RawMessage `json:"-"`
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
	CampaignID       string
	Name             string
	PromotionType    string
	PlacementType    string
	Placements       []string
	LocationIDs      []string
	BudgetMode       string
	Budget           float64
	ScheduleType     string
	ScheduleStart    string
	ScheduleEnd      string
	OptimizationGoal string
	BillingEvent     string
	BidType          string
	BidPrice         float64
	RequestID        string
	Fields           map[string]any
}

type UpdateAdGroupRequest struct {
	Name   *string
	Budget *float64
	Fields map[string]any
}

type ListAdGroupsRequest struct {
	IDs             []string
	CampaignIDs     []string
	Name            string
	PrimaryStatus   string
	SecondaryStatus string
	Fields          []string
	Page            int
	PageSize        int
}

type Ad struct {
	ID                     string          `json:"ad_id"`
	AdvertiserID           string          `json:"advertiser_id,omitempty"`
	CampaignID             string          `json:"campaign_id,omitempty"`
	CampaignName           string          `json:"campaign_name,omitempty"`
	AdGroupID              string          `json:"adgroup_id,omitempty"`
	AdGroupName            string          `json:"adgroup_name,omitempty"`
	Name                   string          `json:"ad_name,omitempty"`
	IdentityType           string          `json:"identity_type,omitempty"`
	IdentityID             string          `json:"identity_id,omitempty"`
	IdentityAuthorizedBCID string          `json:"identity_authorized_bc_id,omitempty"`
	AdFormat               string          `json:"ad_format,omitempty"`
	VideoID                string          `json:"video_id,omitempty"`
	ImageIDs               []string        `json:"image_ids,omitempty"`
	TikTokItemID           string          `json:"tiktok_item_id,omitempty"`
	AdText                 string          `json:"ad_text,omitempty"`
	CallToAction           string          `json:"call_to_action,omitempty"`
	LandingPageURL         string          `json:"landing_page_url,omitempty"`
	OperationStatus        OperationStatus `json:"operation_status,omitempty"`
	SecondaryStatus        string          `json:"secondary_status,omitempty"`
	CreateTime             string          `json:"create_time,omitempty"`
	ModifyTime             string          `json:"modify_time,omitempty"`
	Raw                    json.RawMessage `json:"-"`
}

func (value *Ad) UnmarshalJSON(data []byte) error {
	type alias Ad
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Ad(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type AdCreative struct {
	Name                   string
	IdentityType           string
	IdentityID             string
	IdentityAuthorizedBCID string
	AdFormat               string
	VideoID                string
	ImageIDs               []string
	TikTokItemID           string
	AdText                 string
	CallToAction           string
	LandingPageURL         string
	Fields                 map[string]any
}

type CreateAdsRequest struct {
	AdGroupID string
	Creatives []AdCreative
}

type ListAdsRequest struct {
	IDs             []string
	CampaignIDs     []string
	AdGroupIDs      []string
	PrimaryStatus   string
	SecondaryStatus string
	Fields          []string
	Page            int
	PageSize        int
}

type BatchError struct {
	ID      string `json:"id,omitempty"`
	Message string `json:"error_message,omitempty"`
}

type BatchResult struct {
	SucceededIDs []string
	Errors       []BatchError
}

type ReportDataLevel string

const (
	ReportLevelAdvertiser ReportDataLevel = "AUCTION_ADVERTISER"
	ReportLevelCampaign   ReportDataLevel = "AUCTION_CAMPAIGN"
	ReportLevelAdGroup    ReportDataLevel = "AUCTION_ADGROUP"
	ReportLevelAd         ReportDataLevel = "AUCTION_AD"
)

type ReportFilter struct {
	FieldName   string `json:"field_name"`
	FilterType  string `json:"filter_type"`
	FilterValue string `json:"filter_value"`
}

type ReportRequest struct {
	DataLevel  ReportDataLevel
	StartDate  string
	EndDate    string
	Dimensions []string
	Metrics    []string
	Filtering  []ReportFilter
	Page       int
	PageSize   int
}

type ReportRow struct {
	Dimensions map[string]json.RawMessage `json:"dimensions,omitempty"`
	Metrics    map[string]json.RawMessage `json:"metrics,omitempty"`
	Raw        json.RawMessage            `json:"-"`
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

type AuthorizationRequest struct {
	RedirectURI string
	State       string
}

type OAuthToken struct {
	Token         socialhub.Token
	AdvertiserIDs []string
	ScopeIDs      []int64
}

type AdvertiserWorkflow interface {
	GetAdvertiser(context.Context, ...socialhub.CallOption) (*Advertiser, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (NumberPage[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignStatus(context.Context, string, OperationStatus, ...socialhub.CallOption) (BatchResult, error)
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListAdGroupsRequest, ...socialhub.CallOption) (NumberPage[AdGroup], error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, string, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupStatus(context.Context, string, OperationStatus, ...socialhub.CallOption) (BatchResult, error)
}

type AdWorkflow interface {
	ListAds(context.Context, ListAdsRequest, ...socialhub.CallOption) (NumberPage[Ad], error)
	CreateAds(context.Context, CreateAdsRequest, ...socialhub.CallOption) ([]Ad, error)
	SetAdStatus(context.Context, string, OperationStatus, ...socialhub.CallOption) (BatchResult, error)
}

type ReportWorkflow interface {
	GetReport(context.Context, ReportRequest, ...socialhub.CallOption) (NumberPage[ReportRow], error)
}
