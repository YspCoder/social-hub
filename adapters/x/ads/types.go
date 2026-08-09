package ads

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type EntityStatus string

const (
	StatusActive EntityStatus = "ACTIVE"
	StatusDraft  EntityStatus = "DRAFT"
	StatusPaused EntityStatus = "PAUSED"
)

type Objective string

const (
	ObjectiveAppEngagements Objective = "APP_ENGAGEMENTS"
	ObjectiveAppInstalls    Objective = "APP_INSTALLS"
	ObjectiveReach          Objective = "REACH"
	ObjectiveFollowers      Objective = "FOLLOWERS"
	ObjectiveEngagements    Objective = "ENGAGEMENTS"
	ObjectiveVideoViews     Objective = "VIDEO_VIEWS"
	ObjectivePrerollViews   Objective = "PREROLL_VIEWS"
	ObjectiveWebsiteClicks  Objective = "WEBSITE_CLICKS"
)

type ProductType string

const (
	ProductMedia           ProductType = "MEDIA"
	ProductPromotedAccount ProductType = "PROMOTED_ACCOUNT"
	ProductPromotedTweets  ProductType = "PROMOTED_TWEETS"
)

type Placement string

const (
	PlacementAllOnTwitter     Placement = "ALL_ON_TWITTER"
	PlacementPublisherNetwork Placement = "PUBLISHER_NETWORK"
	PlacementTapBanner        Placement = "TAP_BANNER"
	PlacementTapFull          Placement = "TAP_FULL"
	PlacementTapFullLandscape Placement = "TAP_FULL_LANDSCAPE"
	PlacementTapNative        Placement = "TAP_NATIVE"
	PlacementTapMRect         Placement = "TAP_MRECT"
	PlacementTwitterProfile   Placement = "TWITTER_PROFILE"
	PlacementTwitterReplies   Placement = "TWITTER_REPLIES"
	PlacementTwitterSearch    Placement = "TWITTER_SEARCH"
	PlacementTwitterTimeline  Placement = "TWITTER_TIMELINE"
)

type BidStrategy string

const (
	BidStrategyAuto   BidStrategy = "AUTO"
	BidStrategyMax    BidStrategy = "MAX"
	BidStrategyTarget BidStrategy = "TARGET"
)

type AnalyticsEntity string

const (
	AnalyticsAccount           AnalyticsEntity = "ACCOUNT"
	AnalyticsCampaign          AnalyticsEntity = "CAMPAIGN"
	AnalyticsFundingInstrument AnalyticsEntity = "FUNDING_INSTRUMENT"
	AnalyticsLineItem          AnalyticsEntity = "LINE_ITEM"
	AnalyticsPromotedAccount   AnalyticsEntity = "PROMOTED_ACCOUNT"
	AnalyticsPromotedTweet     AnalyticsEntity = "PROMOTED_TWEET"
)

type Granularity string

const (
	GranularityDay   Granularity = "DAY"
	GranularityHour  Granularity = "HOUR"
	GranularityTotal Granularity = "TOTAL"
)

type AnalyticsPlacement string

const (
	AnalyticsPlacementAllOnTwitter AnalyticsPlacement = "ALL_ON_TWITTER"
	AnalyticsPlacementSpotlight    AnalyticsPlacement = "SPOTLIGHT"
	AnalyticsPlacementTrend        AnalyticsPlacement = "TREND"
)

type MetricGroup string

const (
	MetricGroupBilling                  MetricGroup = "BILLING"
	MetricGroupEngagement               MetricGroup = "ENGAGEMENT"
	MetricGroupLifetimeMobileConversion MetricGroup = "LIFE_TIME_VALUE_MOBILE_CONVERSION"
	MetricGroupMobileConversion         MetricGroup = "MOBILE_CONVERSION"
	MetricGroupVideo                    MetricGroup = "VIDEO"
	MetricGroupWebConversion            MetricGroup = "WEB_CONVERSION"
)

type AdAccount struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	BusinessName   string `json:"business_name,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	ApprovalStatus string `json:"approval_status,omitempty"`
	BusinessID     string `json:"business_id,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	Deleted        bool   `json:"deleted,omitempty"`
}

type AuthenticatedUserAccess struct {
	UserID      string   `json:"user_id"`
	Permissions []string `json:"permissions"`
}

type FundingInstrument struct {
	ID           string       `json:"id"`
	AccountID    string       `json:"account_id,omitempty"`
	Name         string       `json:"name,omitempty"`
	Currency     string       `json:"currency,omitempty"`
	EntityStatus EntityStatus `json:"entity_status,omitempty"`
	AbleToFund   bool         `json:"able_to_fund,omitempty"`
	Deleted      bool         `json:"deleted,omitempty"`
}

type Campaign struct {
	ID                          string       `json:"id"`
	AccountID                   string       `json:"account_id,omitempty"`
	FundingInstrumentID         string       `json:"funding_instrument_id,omitempty"`
	Name                        string       `json:"name,omitempty"`
	EntityStatus                EntityStatus `json:"entity_status,omitempty"`
	EffectiveStatus             string       `json:"effective_status,omitempty"`
	BudgetOptimization          string       `json:"budget_optimization,omitempty"`
	DailyBudgetAmountLocalMicro *int64       `json:"daily_budget_amount_local_micro,omitempty"`
	TotalBudgetAmountLocalMicro *int64       `json:"total_budget_amount_local_micro,omitempty"`
	Currency                    string       `json:"currency,omitempty"`
	CreatedAt                   string       `json:"created_at,omitempty"`
	UpdatedAt                   string       `json:"updated_at,omitempty"`
	Deleted                     bool         `json:"deleted,omitempty"`
}

type LineItem struct {
	ID                          string       `json:"id"`
	AccountID                   string       `json:"account_id,omitempty"`
	CampaignID                  string       `json:"campaign_id"`
	Name                        string       `json:"name,omitempty"`
	Objective                   Objective    `json:"objective,omitempty"`
	ProductType                 ProductType  `json:"product_type,omitempty"`
	Placements                  []Placement  `json:"placements,omitempty"`
	BidStrategy                 BidStrategy  `json:"bid_strategy,omitempty"`
	BidAmountLocalMicro         *int64       `json:"bid_amount_local_micro,omitempty"`
	DailyBudgetAmountLocalMicro *int64       `json:"daily_budget_amount_local_micro,omitempty"`
	TotalBudgetAmountLocalMicro *int64       `json:"total_budget_amount_local_micro,omitempty"`
	StartTime                   string       `json:"start_time,omitempty"`
	EndTime                     string       `json:"end_time,omitempty"`
	EntityStatus                EntityStatus `json:"entity_status,omitempty"`
	Currency                    string       `json:"currency,omitempty"`
	CreatedAt                   string       `json:"created_at,omitempty"`
	UpdatedAt                   string       `json:"updated_at,omitempty"`
	Deleted                     bool         `json:"deleted,omitempty"`
}

type PromotedTweet struct {
	ID             string       `json:"id"`
	LineItemID     string       `json:"line_item_id"`
	TweetID        string       `json:"tweet_id"`
	EntityStatus   EntityStatus `json:"entity_status,omitempty"`
	ApprovalStatus string       `json:"approval_status,omitempty"`
	CreatedAt      string       `json:"created_at,omitempty"`
	UpdatedAt      string       `json:"updated_at,omitempty"`
	Deleted        bool         `json:"deleted,omitempty"`
}

type ListRequest struct {
	Cursor string
	Count  int
}

type CreateCampaignRequest struct {
	FundingInstrumentID         string
	Name                        string
	DailyBudgetAmountLocalMicro int64
	TotalBudgetAmountLocalMicro *int64
}

type UpdateCampaignRequest struct {
	Name                        *string
	Status                      *EntityStatus
	DailyBudgetAmountLocalMicro *int64
	TotalBudgetAmountLocalMicro *int64
}

type CreateLineItemRequest struct {
	CampaignID                  string
	Name                        string
	Objective                   Objective
	ProductType                 ProductType
	Placements                  []Placement
	BidStrategy                 BidStrategy
	BidAmountLocalMicro         *int64
	DailyBudgetAmountLocalMicro *int64
	TotalBudgetAmountLocalMicro *int64
	StartTime                   time.Time
	EndTime                     *time.Time
}

type UpdateLineItemRequest struct {
	Name                        *string
	Status                      *EntityStatus
	BidAmountLocalMicro         *int64
	DailyBudgetAmountLocalMicro *int64
	TotalBudgetAmountLocalMicro *int64
}

type ListPromotedTweetsRequest struct {
	Cursor      string
	Count       int
	LineItemIDs []string
}

type AssociateTweetsRequest struct {
	LineItemID string
	TweetIDs   []string
}

type StatsRequest struct {
	Entity       AnalyticsEntity
	EntityIDs    []string
	StartTime    time.Time
	EndTime      time.Time
	Granularity  Granularity
	Placement    AnalyticsPlacement
	MetricGroups []MetricGroup
}

// MetricValues preserves X metric names, nulls, arrays, and integer precision.
type MetricValues map[string]json.RawMessage

type StatsIDData struct {
	Segment json.RawMessage `json:"segment"`
	Metrics MetricValues    `json:"metrics"`
}

type EntityStats struct {
	ID     string        `json:"id"`
	IDData []StatsIDData `json:"id_data"`
}

type StatsResult struct {
	DataType         string
	TimeSeriesLength int
	Entities         []EntityStats
}

type AccountWorkflow interface {
	GetAdAccount(context.Context, ...socialhub.CallOption) (*AdAccount, error)
	GetAuthenticatedUserAccess(context.Context, ...socialhub.CallOption) (*AuthenticatedUserAccess, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[Campaign], error)
	GetCampaign(context.Context, string, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
}

type LineItemWorkflow interface {
	ListLineItems(context.Context, ListRequest, ...socialhub.CallOption) (socialhub.Page[LineItem], error)
	GetLineItem(context.Context, string, ...socialhub.CallOption) (*LineItem, error)
	CreateLineItem(context.Context, CreateLineItemRequest, ...socialhub.CallOption) (*LineItem, error)
	UpdateLineItem(context.Context, string, UpdateLineItemRequest, ...socialhub.CallOption) (*LineItem, error)
}

type PromotedTweetWorkflow interface {
	ListPromotedTweets(context.Context, ListPromotedTweetsRequest, ...socialhub.CallOption) (socialhub.Page[PromotedTweet], error)
	GetPromotedTweet(context.Context, string, ...socialhub.CallOption) (*PromotedTweet, error)
	AssociateTweets(context.Context, AssociateTweetsRequest, ...socialhub.CallOption) ([]PromotedTweet, error)
}

type StatsWorkflow interface {
	GetStats(context.Context, StatsRequest, ...socialhub.CallOption) (StatsResult, error)
}

type singleResponse[T any] struct {
	Data T `json:"data"`
}

type listResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}
