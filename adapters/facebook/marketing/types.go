package marketing

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// ManagementWorkflow exposes Meta's spend-affecting advertising resources.
// Create methods always create paused resources; activation is an explicit
// update operation.
type ManagementWorkflow interface {
	GetAdAccount(context.Context, ...socialhub.CallOption) (*AdAccount, error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (socialhub.Page[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) error
	GetAdSet(context.Context, string, ...socialhub.CallOption) (*AdSet, error)
	ListAdSets(context.Context, ListAdSetsRequest, ...socialhub.CallOption) (socialhub.Page[AdSet], error)
	CreateAdSet(context.Context, CreateAdSetRequest, ...socialhub.CallOption) (*AdSet, error)
	UpdateAdSet(context.Context, string, UpdateAdSetRequest, ...socialhub.CallOption) error
	GetAdCreative(context.Context, string, ...socialhub.CallOption) (*AdCreative, error)
	ListAdCreatives(context.Context, ListAdCreativesRequest, ...socialhub.CallOption) (socialhub.Page[AdCreative], error)
	CreateAdCreative(context.Context, CreateAdCreativeRequest, ...socialhub.CallOption) (*AdCreative, error)
	GetAd(context.Context, string, ...socialhub.CallOption) (*Ad, error)
	ListAds(context.Context, ListAdsRequest, ...socialhub.CallOption) (socialhub.Page[Ad], error)
	CreateAd(context.Context, CreateAdRequest, ...socialhub.CallOption) (*Ad, error)
	UpdateAd(context.Context, string, UpdateAdRequest, ...socialhub.CallOption) error
}

// InsightsWorkflow exposes synchronous Insights reads. Large asynchronous
// report jobs are deliberately outside the initial adapter contract.
type InsightsWorkflow interface {
	GetInsights(context.Context, InsightsRequest, ...socialhub.CallOption) (InsightsPage, error)
}

// GraphTime accepts both RFC3339 and Meta's numeric-offset timestamp format.
type GraphTime struct{ time.Time }

func (value *GraphTime) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05-0700"} {
		parsed, err := time.Parse(layout, text)
		if err == nil {
			value.Time = parsed
			return nil
		}
	}
	return &time.ParseError{Layout: time.RFC3339, Value: text}
}

func (value GraphTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(value.Time.Format(time.RFC3339))
}

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusPaused   Status = "PAUSED"
	StatusDeleted  Status = "DELETED"
	StatusArchived Status = "ARCHIVED"
)

type Objective string

const (
	ObjectiveAwareness    Objective = "OUTCOME_AWARENESS"
	ObjectiveTraffic      Objective = "OUTCOME_TRAFFIC"
	ObjectiveEngagement   Objective = "OUTCOME_ENGAGEMENT"
	ObjectiveLeads        Objective = "OUTCOME_LEADS"
	ObjectiveAppPromotion Objective = "OUTCOME_APP_PROMOTION"
	ObjectiveSales        Objective = "OUTCOME_SALES"
)

type SpecialAdCategory string

const (
	SpecialAdCategoryCredit     SpecialAdCategory = "CREDIT"
	SpecialAdCategoryEmployment SpecialAdCategory = "EMPLOYMENT"
	SpecialAdCategoryHousing    SpecialAdCategory = "HOUSING"
	SpecialAdCategoryPolitics   SpecialAdCategory = "ISSUES_ELECTIONS_POLITICS"
)

type BillingEvent string

const (
	BillingEventImpressions BillingEvent = "IMPRESSIONS"
	BillingEventLinkClicks  BillingEvent = "LINK_CLICKS"
	BillingEventThruPlay    BillingEvent = "THRUPLAY"
	BillingEventAppInstalls BillingEvent = "APP_INSTALLS"
)

type AdAccount struct {
	ID            string          `json:"id"`
	Name          string          `json:"name,omitempty"`
	AccountStatus int             `json:"account_status,omitempty"`
	Currency      string          `json:"currency,omitempty"`
	TimezoneName  string          `json:"timezone_name,omitempty"`
	AmountSpent   string          `json:"amount_spent,omitempty"`
	Balance       string          `json:"balance,omitempty"`
	SpendCap      string          `json:"spend_cap,omitempty"`
	DisableReason int             `json:"disable_reason,omitempty"`
	Business      *Business       `json:"business,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type Business struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

func (value *AdAccount) UnmarshalJSON(data []byte) error {
	type alias AdAccount
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AdAccount(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Campaign struct {
	ID                  string              `json:"id"`
	AccountID           string              `json:"account_id,omitempty"`
	Name                string              `json:"name,omitempty"`
	Objective           Objective           `json:"objective,omitempty"`
	Status              Status              `json:"status,omitempty"`
	ConfiguredStatus    Status              `json:"configured_status,omitempty"`
	EffectiveStatus     string              `json:"effective_status,omitempty"`
	BuyingType          string              `json:"buying_type,omitempty"`
	BidStrategy         string              `json:"bid_strategy,omitempty"`
	DailyBudget         string              `json:"daily_budget,omitempty"`
	LifetimeBudget      string              `json:"lifetime_budget,omitempty"`
	BudgetRemaining     string              `json:"budget_remaining,omitempty"`
	SpecialAdCategories []SpecialAdCategory `json:"special_ad_categories,omitempty"`
	CreatedTime         *GraphTime          `json:"created_time,omitempty"`
	UpdatedTime         *GraphTime          `json:"updated_time,omitempty"`
	Raw                 json.RawMessage     `json:"-"`
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
	Name                string
	Objective           Objective
	BuyingType          string
	BidStrategy         string
	DailyBudget         int64
	LifetimeBudget      int64
	SpecialAdCategories []SpecialAdCategory
}

type UpdateCampaignRequest struct {
	Name                *string
	Status              *Status
	BidStrategy         *string
	DailyBudget         *int64
	LifetimeBudget      *int64
	SpecialAdCategories *[]SpecialAdCategory
}

type ListCampaignsRequest struct {
	Cursor            string
	MaxResults        int
	EffectiveStatuses []string
}

type AdSet struct {
	ID               string          `json:"id"`
	AccountID        string          `json:"account_id,omitempty"`
	CampaignID       string          `json:"campaign_id,omitempty"`
	Name             string          `json:"name,omitempty"`
	Status           Status          `json:"status,omitempty"`
	ConfiguredStatus Status          `json:"configured_status,omitempty"`
	EffectiveStatus  string          `json:"effective_status,omitempty"`
	OptimizationGoal string          `json:"optimization_goal,omitempty"`
	BillingEvent     BillingEvent    `json:"billing_event,omitempty"`
	BidStrategy      string          `json:"bid_strategy,omitempty"`
	BidAmount        string          `json:"bid_amount,omitempty"`
	DailyBudget      string          `json:"daily_budget,omitempty"`
	LifetimeBudget   string          `json:"lifetime_budget,omitempty"`
	BudgetRemaining  string          `json:"budget_remaining,omitempty"`
	StartTime        *GraphTime      `json:"start_time,omitempty"`
	EndTime          *GraphTime      `json:"end_time,omitempty"`
	Targeting        TargetingSpec   `json:"targeting,omitempty"`
	PromotedObject   *PromotedObject `json:"promoted_object,omitempty"`
	CreatedTime      *GraphTime      `json:"created_time,omitempty"`
	UpdatedTime      *GraphTime      `json:"updated_time,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

func (value *AdSet) UnmarshalJSON(data []byte) error {
	type alias AdSet
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AdSet(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateAdSetRequest struct {
	Name             string
	CampaignID       string
	OptimizationGoal string
	BillingEvent     BillingEvent
	BidStrategy      string
	BidAmount        int64
	DailyBudget      int64
	LifetimeBudget   int64
	StartTime        *time.Time
	EndTime          *time.Time
	Targeting        TargetingSpec
	PromotedObject   *PromotedObject
}

type UpdateAdSetRequest struct {
	Name           *string
	Status         *Status
	BidAmount      *int64
	DailyBudget    *int64
	LifetimeBudget *int64
	EndTime        *time.Time
	Targeting      *TargetingSpec
	PromotedObject *PromotedObject
}

type ListAdSetsRequest struct {
	CampaignID        string
	Cursor            string
	MaxResults        int
	EffectiveStatuses []string
}

type TargetingSpec struct {
	AgeMin                   int                 `json:"age_min,omitempty"`
	AgeMax                   int                 `json:"age_max,omitempty"`
	Genders                  []int               `json:"genders,omitempty"`
	GeoLocations             *GeoLocations       `json:"geo_locations,omitempty"`
	ExcludedGeoLocations     *GeoLocations       `json:"excluded_geo_locations,omitempty"`
	CustomAudiences          []TargetRef         `json:"custom_audiences,omitempty"`
	ExcludedCustomAudiences  []TargetRef         `json:"excluded_custom_audiences,omitempty"`
	FlexibleSpec             []FlexibleTargeting `json:"flexible_spec,omitempty"`
	PublisherPlatforms       []string            `json:"publisher_platforms,omitempty"`
	FacebookPositions        []string            `json:"facebook_positions,omitempty"`
	InstagramPositions       []string            `json:"instagram_positions,omitempty"`
	MessengerPositions       []string            `json:"messenger_positions,omitempty"`
	AudienceNetworkPositions []string            `json:"audience_network_positions,omitempty"`
	DevicePlatforms          []string            `json:"device_platforms,omitempty"`
	Locales                  []int               `json:"locales,omitempty"`
}

type GeoLocations struct {
	Countries     []string    `json:"countries,omitempty"`
	LocationTypes []string    `json:"location_types,omitempty"`
	Cities        []GeoTarget `json:"cities,omitempty"`
	Regions       []GeoTarget `json:"regions,omitempty"`
	Zips          []GeoTarget `json:"zips,omitempty"`
}

type GeoTarget struct {
	Key          string `json:"key"`
	Name         string `json:"name,omitempty"`
	Country      string `json:"country,omitempty"`
	Radius       int    `json:"radius,omitempty"`
	DistanceUnit string `json:"distance_unit,omitempty"`
}

type TargetRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type FlexibleTargeting struct {
	Interests     []TargetRef `json:"interests,omitempty"`
	Behaviors     []TargetRef `json:"behaviors,omitempty"`
	LifeEvents    []TargetRef `json:"life_events,omitempty"`
	WorkEmployers []TargetRef `json:"work_employers,omitempty"`
	WorkPositions []TargetRef `json:"work_positions,omitempty"`
}

type PromotedObject struct {
	PageID             string `json:"page_id,omitempty"`
	PixelID            string `json:"pixel_id,omitempty"`
	CustomEventType    string `json:"custom_event_type,omitempty"`
	CustomConversionID string `json:"custom_conversion_id,omitempty"`
}

type AdCreative struct {
	ID                     string          `json:"id"`
	AccountID              string          `json:"account_id,omitempty"`
	Name                   string          `json:"name,omitempty"`
	ObjectStoryID          string          `json:"object_story_id,omitempty"`
	EffectiveObjectStoryID string          `json:"effective_object_story_id,omitempty"`
	ThumbnailURL           string          `json:"thumbnail_url,omitempty"`
	Body                   string          `json:"body,omitempty"`
	Title                  string          `json:"title,omitempty"`
	URLTags                string          `json:"url_tags,omitempty"`
	Raw                    json.RawMessage `json:"-"`
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
	Name            string
	ObjectStoryID   string
	ObjectStorySpec *ObjectStorySpec
	URLTags         string
}

type ObjectStorySpec struct {
	PageID   string    `json:"page_id"`
	LinkData *LinkData `json:"link_data,omitempty"`
}

type LinkData struct {
	Link         string        `json:"link"`
	Message      string        `json:"message,omitempty"`
	Name         string        `json:"name,omitempty"`
	Description  string        `json:"description,omitempty"`
	ImageHash    string        `json:"image_hash,omitempty"`
	CallToAction *CallToAction `json:"call_to_action,omitempty"`
}

type CallToAction struct {
	Type  string            `json:"type"`
	Value CallToActionValue `json:"value"`
}

type CallToActionValue struct {
	Link string `json:"link"`
}

type ListAdCreativesRequest struct {
	Cursor     string
	MaxResults int
}

type Ad struct {
	ID               string          `json:"id"`
	AccountID        string          `json:"account_id,omitempty"`
	CampaignID       string          `json:"campaign_id,omitempty"`
	AdSetID          string          `json:"adset_id,omitempty"`
	Name             string          `json:"name,omitempty"`
	Status           Status          `json:"status,omitempty"`
	ConfiguredStatus Status          `json:"configured_status,omitempty"`
	EffectiveStatus  string          `json:"effective_status,omitempty"`
	Creative         *CreativeRef    `json:"creative,omitempty"`
	CreatedTime      *GraphTime      `json:"created_time,omitempty"`
	UpdatedTime      *GraphTime      `json:"updated_time,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

type CreativeRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
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

type CreateAdRequest struct {
	Name       string
	AdSetID    string
	CreativeID string
}

type UpdateAdRequest struct {
	Name       *string
	Status     *Status
	CreativeID *string
}

type ListAdsRequest struct {
	AdSetID           string
	Cursor            string
	MaxResults        int
	EffectiveStatuses []string
}

type InsightLevel string

const (
	InsightLevelAccount  InsightLevel = "account"
	InsightLevelCampaign InsightLevel = "campaign"
	InsightLevelAdSet    InsightLevel = "adset"
	InsightLevelAd       InsightLevel = "ad"
)

type TimeRange struct {
	Since string `json:"since"`
	Until string `json:"until"`
}

type InsightsRequest struct {
	EntityID      string
	Level         InsightLevel
	Fields        []string
	Breakdowns    []string
	DatePreset    string
	TimeRange     *TimeRange
	TimeIncrement int
	Cursor        string
	MaxResults    int
}

type Insight struct {
	AccountID         string          `json:"account_id,omitempty"`
	AccountName       string          `json:"account_name,omitempty"`
	CampaignID        string          `json:"campaign_id,omitempty"`
	CampaignName      string          `json:"campaign_name,omitempty"`
	AdSetID           string          `json:"adset_id,omitempty"`
	AdSetName         string          `json:"adset_name,omitempty"`
	AdID              string          `json:"ad_id,omitempty"`
	AdName            string          `json:"ad_name,omitempty"`
	DateStart         string          `json:"date_start,omitempty"`
	DateStop          string          `json:"date_stop,omitempty"`
	Impressions       string          `json:"impressions,omitempty"`
	Reach             string          `json:"reach,omitempty"`
	Frequency         string          `json:"frequency,omitempty"`
	Clicks            string          `json:"clicks,omitempty"`
	Spend             string          `json:"spend,omitempty"`
	CTR               string          `json:"ctr,omitempty"`
	CPC               string          `json:"cpc,omitempty"`
	CPM               string          `json:"cpm,omitempty"`
	Actions           []InsightAction `json:"actions,omitempty"`
	CostPerActionType []InsightAction `json:"cost_per_action_type,omitempty"`
	Raw               json.RawMessage `json:"-"`
}

func (value *Insight) UnmarshalJSON(data []byte) error {
	type alias Insight
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Insight(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type InsightAction struct {
	ActionType string `json:"action_type"`
	Value      string `json:"value"`
	View1d     string `json:"1d_view,omitempty"`
	Click1d    string `json:"1d_click,omitempty"`
}

type InsightsPage struct {
	Items      []Insight `json:"items"`
	NextCursor *string   `json:"next_cursor,omitempty"`
	PrevCursor *string   `json:"prev_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
	Summary    *Insight  `json:"summary,omitempty"`
}

type graphPaging struct {
	Cursors struct {
		Before *string `json:"before"`
		After  *string `json:"after"`
	} `json:"cursors"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
}

type graphPage[T any] struct {
	Data    []T         `json:"data"`
	Paging  graphPaging `json:"paging"`
	Summary *Insight    `json:"summary,omitempty"`
}

type idResponse struct {
	ID                     string `json:"id"`
	EffectiveObjectStoryID string `json:"effective_object_story_id"`
}

type successResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}
