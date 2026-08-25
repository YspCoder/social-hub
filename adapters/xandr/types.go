package xandr

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type State string

const (
	StateActive   State = "active"
	StateInactive State = "inactive"
)

// ResponseMeta preserves current rate-limit and request diagnostics.
type ResponseMeta struct {
	RateLimitCode  string
	RateLimitCount string
	RequestID      string
	RetryAfter     time.Duration
}

// Advertiser contains the stable Digital Platform API advertiser fields.
// Budget fields remain raw JSON so their exact provider representation is not
// rounded or rejected when an account returns null or a quoted decimal.
type Advertiser struct {
	ID                 int64           `json:"id"`
	Code               *string         `json:"code"`
	Name               string          `json:"name"`
	LegalEntityName    *string         `json:"legal_entity_name"`
	State              State           `json:"state"`
	DefaultCurrency    *string         `json:"default_currency"`
	Timezone           *string         `json:"timezone"`
	DailyBudget        json.RawMessage `json:"daily_budget"`
	DailyBudgetImps    json.RawMessage `json:"daily_budget_imps"`
	LifetimeBudget     json.RawMessage `json:"lifetime_budget"`
	LifetimeBudgetImps json.RawMessage `json:"lifetime_budget_imps"`
	EnablePacing       bool            `json:"enable_pacing"`
	ProfileID          *int64          `json:"profile_id"`
	LastModified       *string         `json:"last_modified"`
	UseInsertionOrders bool            `json:"use_insertion_orders"`
	Raw                json.RawMessage `json:"-"`
	Meta               ResponseMeta    `json:"-"`
}

// Campaign contains the stable Digital Platform API campaign fields.
type Campaign struct {
	ID                 int64           `json:"id"`
	State              State           `json:"state"`
	ParentInactive     bool            `json:"parent_inactive"`
	Code               *string         `json:"code"`
	Name               string          `json:"name"`
	ShortName          *string         `json:"short_name"`
	AdvertiserID       int64           `json:"advertiser_id"`
	LineItemID         *int64          `json:"line_item_id"`
	ProfileID          *int64          `json:"profile_id"`
	StartDate          *string         `json:"start_date"`
	EndDate            *string         `json:"end_date"`
	Timezone           *string         `json:"timezone"`
	Priority           *int64          `json:"priority"`
	InventoryType      *string         `json:"inventory_type"`
	DailyBudget        json.RawMessage `json:"daily_budget"`
	DailyBudgetImps    json.RawMessage `json:"daily_budget_imps"`
	LifetimeBudget     json.RawMessage `json:"lifetime_budget"`
	LifetimeBudgetImps json.RawMessage `json:"lifetime_budget_imps"`
	EnablePacing       bool            `json:"enable_pacing"`
	LastModified       *string         `json:"last_modified"`
	Raw                json.RawMessage `json:"-"`
	Meta               ResponseMeta    `json:"-"`
}

// ListOptions exposes the documented common read filters. NumElements defaults
// to 100 and cannot exceed the Digital Platform API maximum of 100.
type ListOptions struct {
	State        State
	Search       string
	StartElement int
	NumElements  int
}

type AdvertiserPage struct {
	Advertisers      []Advertiser
	Count            int64
	StartElement     int
	NumElements      int
	HasMore          bool
	NextStartElement *int
	Meta             ResponseMeta
}

type CampaignPage struct {
	Campaigns        []Campaign
	Count            int64
	StartElement     int
	NumElements      int
	HasMore          bool
	NextStartElement *int
	Meta             ResponseMeta
}

type AdvertiserWorkflow interface {
	GetAdvertiser(context.Context, int64, ...socialhub.CallOption) (*Advertiser, error)
	ListAdvertisers(context.Context, ListOptions, ...socialhub.CallOption) (*AdvertiserPage, error)
}

type CampaignWorkflow interface {
	GetCampaign(context.Context, int64, int64, ...socialhub.CallOption) (*Campaign, error)
	ListCampaigns(context.Context, int64, ListOptions, ...socialhub.CallOption) (*CampaignPage, error)
}
