package oceanengine

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type Operation string

const (
	OperationDisable Operation = "DISABLE"
	OperationEnable  Operation = "ENABLE"
)

type AdType string

const (
	AdTypeAll    AdType = "ALL"
	AdTypeSearch AdType = "SEARCH"
)

type LandingType string

const (
	LandingTypeApp          LandingType = "APP"
	LandingTypeDPA          LandingType = "DPA"
	LandingTypeLink         LandingType = "LINK"
	LandingTypeMicroGame    LandingType = "MICRO_GAME"
	LandingTypeNativeAction LandingType = "NATIVE_ACTION"
	LandingTypeQuickApp     LandingType = "QUICK_APP"
	LandingTypeShop         LandingType = "SHOP"
)

type MarketingGoal string

const (
	MarketingGoalLive          MarketingGoal = "LIVE"
	MarketingGoalVideoAndImage MarketingGoal = "VIDEO_AND_IMAGE"
)

type InventoryCatalog string

const (
	InventoryCatalogManual         InventoryCatalog = "MANUAL"
	InventoryCatalogUniversalSmart InventoryCatalog = "UNIVERSAL_SMART"
)

type BidType string

const (
	BidTypeCustom       BidType = "CUSTOM"
	BidTypeNoBid        BidType = "NO_BID"
	BidTypeOptimalCost  BidType = "OPTIMAL_COST"
	BidTypeUpperControl BidType = "UPPER_CONTROL"
)

type BudgetMode string

const (
	BudgetModeDay      BudgetMode = "BUDGET_MODE_DAY"
	BudgetModeInfinite BudgetMode = "BUDGET_MODE_INFINITE"
	BudgetModeTotal    BudgetMode = "BUDGET_MODE_TOTAL"
)

type DeliveryRange struct {
	InventoryCatalog InventoryCatalog `json:"inventory_catalog"`
	InventoryType    []string         `json:"inventory_type,omitempty"`
	UnionVideoType   string           `json:"union_video_type,omitempty"`
}

type DeliverySetting struct {
	BidType      BidType    `json:"bid_type"`
	BudgetMode   BudgetMode `json:"budget_mode"`
	Bid          *float64   `json:"bid,omitempty"`
	Budget       *float64   `json:"budget,omitempty"`
	CPABid       *float64   `json:"cpa_bid,omitempty"`
	DeepBidType  string     `json:"deep_bid_type,omitempty"`
	DeepCPABid   *float64   `json:"deep_cpabid,omitempty"`
	ROIGoal      *float64   `json:"roi_goal,omitempty"`
	ScheduleType string     `json:"schedule_type,omitempty"`
	ScheduleTime string     `json:"schedule_time,omitempty"`
	StartTime    string     `json:"start_time,omitempty"`
	EndTime      string     `json:"end_time,omitempty"`
}

// CreateProjectRequest contains the stable required v3 fields. Fields exposes
// conditional provider fields while reserved account and activation keys stay
// under adapter control.
type CreateProjectRequest struct {
	Name            string
	AdType          AdType
	LandingType     LandingType
	MarketingGoal   MarketingGoal
	DeliveryRange   DeliveryRange
	DeliverySetting DeliverySetting
	Fields          map[string]any
}

type UpdateProjectRequest struct {
	Name   *string
	Fields map[string]any
}

type ProjectFilter struct {
	IDs    []int64
	Name   string
	Status string
}

type ListProjectsRequest struct {
	Fields   []string
	Filter   ProjectFilter
	Page     int
	PageSize int
}

type Project struct {
	ID              int64           `json:"project_id"`
	AdvertiserID    int64           `json:"advertiser_id"`
	Name            string          `json:"name,omitempty"`
	AdType          AdType          `json:"ad_type,omitempty"`
	LandingType     LandingType     `json:"landing_type,omitempty"`
	MarketingGoal   MarketingGoal   `json:"marketing_goal,omitempty"`
	OptStatus       string          `json:"opt_status,omitempty"`
	Status          string          `json:"status,omitempty"`
	StatusFirst     string          `json:"status_first,omitempty"`
	StatusSecond    json.RawMessage `json:"status_second,omitempty"`
	CreatedTime     string          `json:"project_create_time,omitempty"`
	ModifiedTime    string          `json:"project_modify_time,omitempty"`
	DeliveryRange   json.RawMessage `json:"delivery_range,omitempty"`
	DeliverySetting json.RawMessage `json:"delivery_setting,omitempty"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Project) UnmarshalJSON(data []byte) error {
	type alias Project
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Project(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreatePromotionRequest struct {
	ProjectID int64
	Name      string
	Fields    map[string]any
}

type UpdatePromotionRequest struct {
	Name   string
	Fields map[string]any
}

type PromotionFilter struct {
	IDs       []int64
	ProjectID int64
	Name      string
	Status    string
}

type ListPromotionsRequest struct {
	Fields   []string
	Filter   PromotionFilter
	Page     int
	PageSize int
}

type Promotion struct {
	ID           int64           `json:"promotion_id"`
	AdvertiserID int64           `json:"advertiser_id"`
	ProjectID    int64           `json:"project_id"`
	Name         string          `json:"promotion_name,omitempty"`
	OptStatus    string          `json:"opt_status,omitempty"`
	Status       string          `json:"status,omitempty"`
	StatusFirst  string          `json:"status_first,omitempty"`
	StatusSecond json.RawMessage `json:"status_second,omitempty"`
	Budget       *float64        `json:"budget,omitempty"`
	BudgetMode   string          `json:"budget_mode,omitempty"`
	Bid          *float64        `json:"bid,omitempty"`
	CPABid       *float64        `json:"cpa_bid,omitempty"`
	CreatedTime  string          `json:"promotion_create_time,omitempty"`
	ModifiedTime string          `json:"promotion_modify_time,omitempty"`
	Materials    json.RawMessage `json:"promotion_materials,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

func (value *Promotion) UnmarshalJSON(data []byte) error {
	type alias Promotion
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Promotion(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type NumberPage[T any] struct {
	Items       []T
	Page        int
	PageSize    int
	TotalNumber int64
	TotalPages  int
	HasMore     bool
}

type ReportDataTopic string

const (
	ReportTopicBasicData      ReportDataTopic = "BASIC_DATA"
	ReportTopicMaterialData   ReportDataTopic = "MATERIAL_DATA"
	ReportTopicProductData    ReportDataTopic = "PRODUCT_DATA"
	ReportTopicQueryData      ReportDataTopic = "QUERY_DATA"
	ReportTopicBidwordData    ReportDataTopic = "BIDWORD_DATA"
	ReportTopicUniProjectData ReportDataTopic = "UNI_PROJECT_DATA"
)

type ReportFilter struct {
	Field    string   `json:"field"`
	Operator *int64   `json:"operator,omitempty"`
	Type     int64    `json:"type"`
	Values   []string `json:"values"`
}

type ReportOrder struct {
	Field string `json:"field"`
	Type  string `json:"type,omitempty"`
}

type CustomReportRequest struct {
	Dimensions []string
	Metrics    []string
	Filters    []ReportFilter
	StartTime  string
	EndTime    string
	OrderBy    []ReportOrder
	DataTopic  ReportDataTopic
	Page       int
	PageSize   int
}

type ReportRow struct {
	Dimensions map[string]string `json:"dimensions"`
	Metrics    map[string]string `json:"metrics"`
}

type CustomReportPage struct {
	NumberPage[ReportRow]
	TotalMetrics map[string]string
}

type OAuthToken struct {
	Token            socialhub.Token
	AdvertiserIDs    []int64
	RefreshExpiresAt time.Time
}

type ProjectWorkflow interface {
	ListProjects(context.Context, ListProjectsRequest, ...socialhub.CallOption) (NumberPage[Project], error)
	CreateProject(context.Context, CreateProjectRequest, ...socialhub.CallOption) (*Project, error)
	UpdateProject(context.Context, int64, UpdateProjectRequest, ...socialhub.CallOption) error
	SetProjectStatus(context.Context, int64, Operation, ...socialhub.CallOption) error
}

type PromotionWorkflow interface {
	ListPromotions(context.Context, ListPromotionsRequest, ...socialhub.CallOption) (NumberPage[Promotion], error)
	CreatePromotion(context.Context, CreatePromotionRequest, ...socialhub.CallOption) (*Promotion, error)
	UpdatePromotion(context.Context, int64, UpdatePromotionRequest, ...socialhub.CallOption) error
	SetPromotionStatus(context.Context, int64, Operation, ...socialhub.CallOption) error
}

type ReportWorkflow interface {
	GetCustomReport(context.Context, CustomReportRequest, ...socialhub.CallOption) (CustomReportPage, error)
}
