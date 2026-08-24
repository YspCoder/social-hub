package xiaohongshumarketing

import (
	"context"
	"encoding/json"
	"errors"

	"social-hub/pkg/socialhub"
)

var (
	// ErrOutcomeUnknown means a write was dispatched but no definitive response
	// was received. Reconcile provider state before retrying.
	ErrOutcomeUnknown = errors.New("xiaohongshumarketing: mutation outcome unknown")
	// ErrPartialMutation means Spotlight acknowledged only a subset of a batch.
	ErrPartialMutation = errors.New("xiaohongshumarketing: mutation partially acknowledged")
)

type Date string
type MarketingTarget int
type StatusAction int

const (
	MarketingTargetProductSeeding MarketingTarget = 4
	MarketingTargetLeadGeneration MarketingTarget = 9
)

const (
	StatusActionResume StatusAction = 1
	StatusActionPause  StatusAction = 2
	StatusActionDelete StatusAction = 3
)

// NumberPage normalizes Spotlight's page_index and total_count responses.
type NumberPage[T any] struct {
	Items       []T
	Page        int
	PageSize    int
	TotalNumber int64
	HasMore     bool
	RequestID   string
}

// MutationResult records the requested and provider-acknowledged resource IDs.
// A partial result is returned together with ErrPartialMutation.
type MutationResult struct {
	RequestedIDs    []uint64
	AcknowledgedIDs []uint64
	RequestID       string
}

type TimePeriod struct {
	Mon  string `json:"mon"`
	Tues string `json:"tues"`
	Wed  string `json:"wed"`
	Thur string `json:"thur"`
	Fri  string `json:"fri"`
	Sat  string `json:"sat"`
	Sun  string `json:"sun"`
}

type ExploreConfig struct {
	DayBudgetCents int64       `json:"campaign_day_budget,omitempty"`
	TimePeriod     *TimePeriod `json:"time_period,omitempty"`
	TimePeriodType int         `json:"time_period_type,omitempty"`
	StartTimeMS    int64       `json:"start_time,omitempty"`
	ExpireHours    int64       `json:"expire_hour,omitempty"`
}

// UpdateExploreConfig is a partial one-click acceleration configuration.
// Pointer fields preserve the distinction between omitted and explicit zero.
type UpdateExploreConfig struct {
	DayBudgetCents *int64      `json:"campaign_day_budget,omitempty"`
	TimePeriod     *TimePeriod `json:"time_period,omitempty"`
	TimePeriodType *int        `json:"time_period_type,omitempty"`
	StartTimeMS    *int64      `json:"start_time,omitempty"`
	ExpireHours    *int64      `json:"expire_hour,omitempty"`
}

type ListCampaignsRequest struct {
	IDs             []uint64
	StartDate       Date
	EndDate         Date
	UpdateStartDate Date
	UpdateEndDate   Date
	Status          int
	Page            int
	PageSize        int
}

// UpdateCampaignRequest uses the current cascade/modify contract. The provider
// currently limits this contract to product-seeding and lead-generation goals.
type UpdateCampaignRequest struct {
	MarketingTarget MarketingTarget
	Name            *string
	TimeType        *int
	StartDate       *Date
	EndDate         *Date
	TimePeriodType  *int
	TimePeriod      *TimePeriod
	LimitDayBudget  *int
	DayBudgetCents  *int64
	SmartSwitch     *int
	ExploreState    *int
	ExploreConfig   *UpdateExploreConfig
	SearchFlag      *int
}

type Campaign struct {
	ID                    uint64          `json:"campaign_id"`
	Name                  string          `json:"campaign_name,omitempty"`
	FilterState           int             `json:"campaign_filter_state,omitempty"`
	CreatedTime           string          `json:"campaign_create_time,omitempty"`
	UpdatedTime           string          `json:"campaign_update_time,omitempty"`
	Enable                int             `json:"campaign_enable,omitempty"`
	MarketingTarget       MarketingTarget `json:"marketing_target,omitempty"`
	Placement             int             `json:"placement,omitempty"`
	OptimizeTarget        int             `json:"optimize_target,omitempty"`
	PromotionTarget       int             `json:"promotion_target,omitempty"`
	BiddingStrategy       int             `json:"bidding_strategy,omitempty"`
	ConstraintType        int             `json:"constraint_type,omitempty"`
	ConstraintValue       int64           `json:"constraint_value,omitempty"`
	LimitDayBudget        int             `json:"limit_day_budget,omitempty"`
	DayBudgetCents        int64           `json:"campaign_day_budget,omitempty"`
	BudgetState           int             `json:"budget_state,omitempty"`
	SmartSwitch           int             `json:"smart_switch,omitempty"`
	Platform              int             `json:"platform,omitempty"`
	PacingMode            int             `json:"pacing_mode,omitempty"`
	StartDate             Date            `json:"start_time,omitempty"`
	EndDate               Date            `json:"expire_time,omitempty"`
	TimePeriod            string          `json:"time_period,omitempty"`
	TimePeriodType        int             `json:"time_period_type,omitempty"`
	FeedFlag              int             `json:"feed_flag,omitempty"`
	BuildType             int             `json:"build_type,omitempty"`
	CreativityState       int             `json:"creativity_state,omitempty"`
	EventAssetID          uint64          `json:"event_asset_id,omitempty"`
	AssetEvent            int             `json:"asset_event,omitempty"`
	AssetEventID          uint64          `json:"asset_event_id,omitempty"`
	PageCategory          int             `json:"page_category,omitempty"`
	SearchFlag            int             `json:"search_flag,omitempty"`
	SearchBidRatio        float64         `json:"search_bid_ratio,omitempty"`
	DeeplinkID            uint64          `json:"deeplink_id,omitempty"`
	UniversalLinkID       uint64          `json:"universal_link_id,omitempty"`
	DetectURLLink         string          `json:"detect_url_link,omitempty"`
	NotAvailableStatus    int             `json:"not_available_status,omitempty"`
	OptimizeObjective     int             `json:"optimize_objective,omitempty"`
	DeepOptimizeObjective int             `json:"deep_optimize_objective,omitempty"`
	ExploreConfig         *ExploreConfig  `json:"explore_config,omitempty"`
	CreationType          int             `json:"creation_type,omitempty"`
	MarketingIndustry     int             `json:"marketing_industry,omitempty"`
	Raw                   json.RawMessage `json:"-"`
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

type ListUnitsRequest struct {
	CampaignID      uint64
	IDs             []uint64
	Status          int
	Name            string
	StartDate       Date
	EndDate         Date
	UpdateStartDate Date
	UpdateEndDate   Date
	Page            int
	PageSize        int
}

type Unit struct {
	ID                     uint64          `json:"id"`
	CampaignID             uint64          `json:"campaign_id,omitempty"`
	Name                   string          `json:"name,omitempty"`
	Enable                 int             `json:"enable,omitempty"`
	FilterState            int             `json:"unit_filter_state,omitempty"`
	EventBidCents          int64           `json:"event_bid,omitempty"`
	TargetType             int             `json:"target_type,omitempty"`
	ItemIDs                []string        `json:"item_ids,omitempty"`
	NoteIDs                []string        `json:"note_ids,omitempty"`
	LiveUserID             string          `json:"live_user_id,omitempty"`
	PageID                 string          `json:"page_id,omitempty"`
	LandingPageURL         string          `json:"landing_page_url,omitempty"`
	ExternalPageURL        string          `json:"unit_external_page_url,omitempty"`
	LandingPageType        int             `json:"landing_page_type,omitempty"`
	TargetPosition         int             `json:"target_position,omitempty"`
	TargetGoal             int             `json:"target_goal,omitempty"`
	WordTagName            string          `json:"word_tag_name,omitempty"`
	ProportionGoal         float64         `json:"proportion_goal,omitempty"`
	BusinessTreeName       string          `json:"business_tree_name,omitempty"`
	LandingPageDescription []string        `json:"unit_landing_page_desc,omitempty"`
	KeywordTargetPeriod    int             `json:"keyword_target_period,omitempty"`
	KeywordTargetAction    []int           `json:"keyword_target_action,omitempty"`
	SubstitutedUserID      string          `json:"substituted_user_id,omitempty"`
	CreatedTime            string          `json:"create_time,omitempty"`
	UpdatedTime            string          `json:"update_time,omitempty"`
	ItemNoteInfo           json.RawMessage `json:"item_note_info,omitempty"`
	SPUNoteInfo            json.RawMessage `json:"spu_note_info,omitempty"`
	TargetConfig           json.RawMessage `json:"target_config,omitempty"`
	KeywordGenerationType  int             `json:"keyword_gen_type,omitempty"`
	KeywordWithBids        json.RawMessage `json:"keyword_with_bids,omitempty"`
	NotAvailableStatus     int             `json:"not_available_status,omitempty"`
	AIGCNoteRecommendation int             `json:"aigc_note_black_rec,omitempty"`
	CreationType           int             `json:"creation_type,omitempty"`
	SearchBidRatio         float64         `json:"search_bid_ratio,omitempty"`
	Raw                    json.RawMessage `json:"-"`
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

type SearchCreativesRequest struct {
	CampaignID uint64
	UnitID     uint64
	IDs        []uint64
	Status     int
	StartDate  Date
	EndDate    Date
	NoteID     string
	Page       int
	PageSize   int
}

type Creative struct {
	AdvertiserID             uint64            `json:"advertiser_id,omitempty"`
	CampaignID               uint64            `json:"campaign_id,omitempty"`
	UnitID                   uint64            `json:"unit_id,omitempty"`
	ID                       uint64            `json:"creativity_id"`
	Name                     string            `json:"creativity_name,omitempty"`
	Enable                   int               `json:"creativity_enable,omitempty"`
	FilterState              int               `json:"creativity_filter_state,omitempty"`
	LegacyStatus             int               `json:"creativity_status,omitempty"`
	CreatedTime              string            `json:"creativity_create_time,omitempty"`
	UpdatedTime              string            `json:"creativity_update_time,omitempty"`
	MaterialType             int               `json:"material_type,omitempty"`
	ConversionType           int               `json:"conversion_type,omitempty"`
	NoteID                   string            `json:"note_id,omitempty"`
	NoteType                 int               `json:"note_type,omitempty"`
	CustomMask               int               `json:"custom_mask,omitempty"`
	CustomTitle              int               `json:"custom_title,omitempty"`
	TitleFills               []string          `json:"title_fills,omitempty"`
	MaskGeneration           int               `json:"mask_gen,omitempty"`
	TitleGeneration          int               `json:"title_gen,omitempty"`
	MaskPreferred            bool              `json:"mask_prefer,omitempty"`
	TitlePreferred           bool              `json:"title_mask_prefer,omitempty"`
	AuditStatus              int               `json:"audit_status,omitempty"`
	AuditState               int               `json:"creativity_audit_state,omitempty"`
	AuditComment             map[string]string `json:"audit_comment,omitempty"`
	PageID                   string            `json:"page_id,omitempty"`
	ClickURLs                []string          `json:"click_urls,omitempty"`
	ExposureURLs             []string          `json:"expo_urls,omitempty"`
	JumpURL                  string            `json:"jump_url,omitempty"`
	BarContent               string            `json:"bar_content,omitempty"`
	Image                    string            `json:"image,omitempty"`
	ItemInvalidReason        int               `json:"item_invalid_reason,omitempty"`
	ConversionComponentTypes []int             `json:"conversion_component_types,omitempty"`
	Programmatic             int               `json:"programmatic,omitempty"`
	Comment                  string            `json:"comment,omitempty"`
	ExtraInfo                string            `json:"creativity_extra_info,omitempty"`
	IntoShopParam            string            `json:"into_shop_param,omitempty"`
	BootScreenInfo           json.RawMessage   `json:"boot_screen_info,omitempty"`
	POIID                    string            `json:"poi_id,omitempty"`
	POIJumpType              string            `json:"poi_jump_type,omitempty"`
	MonitorCompany           string            `json:"monitor_company,omitempty"`
	MonitorParams            string            `json:"monitor_params,omitempty"`
	ItemID                   string            `json:"item_id,omitempty"`
	Title                    string            `json:"title,omitempty"`
	GoodsSellingPoint        string            `json:"goods_selling_point,omitempty"`
	DataPostURL              string            `json:"data_post_url,omitempty"`
	KOSMessageType           int               `json:"kos_msg_type,omitempty"`
	QualificationInfo        json.RawMessage   `json:"qual_info,omitempty"`
	MiniProgramPath          string            `json:"mini_program_path,omitempty"`
	PrimaryTitle             string            `json:"primary_title,omitempty"`
	ActionButtonContent      string            `json:"action_button_content,omitempty"`
	HorseRacingResult        string            `json:"horse_racing_result,omitempty"`
	NoteUserID               string            `json:"note_user_id,omitempty"`
	CreationType             int               `json:"creation_type,omitempty"`
	Raw                      json.RawMessage   `json:"-"`
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

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (NumberPage[Campaign], error)
	UpdateCampaign(context.Context, uint64, UpdateCampaignRequest, ...socialhub.CallOption) (MutationResult, error)
	ResumeCampaigns(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
	PauseCampaigns(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
	DeleteCampaigns(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
}

type UnitWorkflow interface {
	ListUnits(context.Context, ListUnitsRequest, ...socialhub.CallOption) (NumberPage[Unit], error)
	ResumeUnits(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
	PauseUnits(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
	DeleteUnits(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
}

type CreativeWorkflow interface {
	SearchCreatives(context.Context, SearchCreativesRequest, ...socialhub.CallOption) (NumberPage[Creative], error)
	ResumeCreatives(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
	PauseCreatives(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
	DeleteCreatives(context.Context, []uint64, ...socialhub.CallOption) (MutationResult, error)
}

var _ CampaignWorkflow = (*Client)(nil)
var _ UnitWorkflow = (*Client)(nil)
var _ CreativeWorkflow = (*Client)(nil)
