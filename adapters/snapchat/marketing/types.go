package marketing

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type EntityStatus string

const (
	StatusActive EntityStatus = "ACTIVE"
	StatusPaused EntityStatus = "PAUSED"
)

type ObjectiveV2Type string

const ObjectiveAwarenessAndEngagement ObjectiveV2Type = "AWARENESS_AND_ENGAGEMENT"

type Granularity string

const (
	GranularityTotal    Granularity = "TOTAL"
	GranularityDay      Granularity = "DAY"
	GranularityHour     Granularity = "HOUR"
	GranularityLifetime Granularity = "LIFETIME"
)

type AdAccount struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id,omitempty"`
	Name           string `json:"name,omitempty"`
	Status         string `json:"status,omitempty"`
	Type           string `json:"type,omitempty"`
	Currency       string `json:"currency,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

type ObjectiveV2Properties struct {
	Type ObjectiveV2Type `json:"objective_v2_type"`
}

type Campaign struct {
	ID                    string                `json:"id"`
	AdAccountID           string                `json:"ad_account_id"`
	Name                  string                `json:"name,omitempty"`
	Status                EntityStatus          `json:"status,omitempty"`
	BuyModel              string                `json:"buy_model,omitempty"`
	CreationState         string                `json:"creation_state,omitempty"`
	StartTime             string                `json:"start_time,omitempty"`
	EndTime               string                `json:"end_time,omitempty"`
	ObjectiveV2Properties ObjectiveV2Properties `json:"objective_v2_properties,omitempty"`
	CreatedAt             string                `json:"created_at,omitempty"`
	UpdatedAt             string                `json:"updated_at,omitempty"`
}

type PlacementV2 struct {
	Config string `json:"config"`
}

type GeoTarget struct {
	CountryCode string `json:"country_code"`
}

type Targeting struct {
	Geos []GeoTarget `json:"geos"`
}

type AdSquad struct {
	ID                 string       `json:"id"`
	AdAccountID        string       `json:"ad_account_id,omitempty"`
	CampaignID         string       `json:"campaign_id"`
	Name               string       `json:"name,omitempty"`
	Status             EntityStatus `json:"status,omitempty"`
	Type               string       `json:"type,omitempty"`
	PlacementV2        PlacementV2  `json:"placement_v2,omitempty"`
	OptimizationGoal   string       `json:"optimization_goal,omitempty"`
	BillingEvent       string       `json:"billing_event,omitempty"`
	BidStrategy        string       `json:"bid_strategy,omitempty"`
	BidMicro           int64        `json:"bid_micro,omitempty"`
	DailyBudgetMicro   int64        `json:"daily_budget_micro,omitempty"`
	DeliveryConstraint string       `json:"delivery_constraint,omitempty"`
	Targeting          Targeting    `json:"targeting,omitempty"`
	StartTime          string       `json:"start_time,omitempty"`
	EndTime            string       `json:"end_time,omitempty"`
	CreatedAt          string       `json:"created_at,omitempty"`
	UpdatedAt          string       `json:"updated_at,omitempty"`
}

type Ad struct {
	ID          string       `json:"id"`
	AdAccountID string       `json:"ad_account_id,omitempty"`
	AdSquadID   string       `json:"ad_squad_id"`
	CreativeID  string       `json:"creative_id"`
	Name        string       `json:"name,omitempty"`
	Type        string       `json:"type,omitempty"`
	Status      EntityStatus `json:"status,omitempty"`
	CreatedAt   string       `json:"created_at,omitempty"`
	UpdatedAt   string       `json:"updated_at,omitempty"`
}

type ListRequest struct {
	Cursor string
	Limit  int
}

type CreateCampaignRequest struct {
	Name      string
	Objective ObjectiveV2Type
	StartTime time.Time
}

type UpdateEntityRequest struct {
	Name   *string
	Status *EntityStatus
}

type CreateAdSquadRequest struct {
	CampaignID       string
	Name             string
	BidMicro         int64
	DailyBudgetMicro int64
	CountryCodes     []string
	StartTime        time.Time
}

type CreateAdRequest struct {
	AdSquadID  string
	CreativeID string
	Name       string
}

type StatsRequest struct {
	Granularity Granularity
	StartTime   *time.Time
	EndTime     *time.Time
	Fields      []string
	Cursor      string
	Limit       int
}

// MetricValues preserves Snapchat metric names and JSON numeric precision.
type MetricValues map[string]json.RawMessage

type TotalStat struct {
	ID          string       `json:"id"`
	Type        string       `json:"type,omitempty"`
	Granularity Granularity  `json:"granularity,omitempty"`
	Stats       MetricValues `json:"stats,omitempty"`
}

type StatsPoint struct {
	StartTime string       `json:"start_time,omitempty"`
	EndTime   string       `json:"end_time,omitempty"`
	Stats     MetricValues `json:"stats,omitempty"`
}

type TimeseriesStat struct {
	ID          string       `json:"id"`
	Type        string       `json:"type,omitempty"`
	Granularity Granularity  `json:"granularity,omitempty"`
	StartTime   string       `json:"start_time,omitempty"`
	EndTime     string       `json:"end_time,omitempty"`
	Timeseries  []StatsPoint `json:"timeseries,omitempty"`
}

type StatsResult struct {
	Totals     []TotalStat
	Timeseries []TimeseriesStat
	NextCursor *string
	HasMore    bool
}

type AdAccountWorkflow interface {
	GetAdAccount(context.Context, ...socialhub.CallOption) (*AdAccount, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[Campaign], error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateEntityRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignStatus(context.Context, string, EntityStatus, ...socialhub.CallOption) (*Campaign, error)
}

type AdSquadWorkflow interface {
	ListAdSquads(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[AdSquad], error)
	GetAdSquad(context.Context, string, ...socialhub.CallOption) (*AdSquad, error)
	CreateAdSquad(context.Context, CreateAdSquadRequest, ...socialhub.CallOption) (*AdSquad, error)
	UpdateAdSquad(context.Context, string, UpdateEntityRequest, ...socialhub.CallOption) (*AdSquad, error)
	SetAdSquadStatus(context.Context, string, EntityStatus, ...socialhub.CallOption) (*AdSquad, error)
}

type AdWorkflow interface {
	ListAds(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[Ad], error)
	GetAd(context.Context, string, ...socialhub.CallOption) (*Ad, error)
	CreateAd(context.Context, CreateAdRequest, ...socialhub.CallOption) (*Ad, error)
	UpdateAd(context.Context, string, UpdateEntityRequest, ...socialhub.CallOption) (*Ad, error)
	SetAdStatus(context.Context, string, EntityStatus, ...socialhub.CallOption) (*Ad, error)
}

type StatsWorkflow interface {
	GetAccountStats(context.Context, StatsRequest, ...socialhub.CallOption) (StatsResult, error)
}

type paging struct {
	NextLink string `json:"next_link"`
}

type jsonPatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}
