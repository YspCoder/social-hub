package unityadvertising

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"social-hub/pkg/socialhub"
)

type Campaign struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	Goal                  CampaignGoal           `json:"goal"`
	BillingType           BillingType            `json:"billingType"`
	Enabled               bool                   `json:"enabled"`
	Status                CampaignStatus         `json:"status,omitempty"`
	AttributionClickURL   *string                `json:"attributionClickUrl"`
	AttributionStartURL   *string                `json:"attributionStartUrl"`
	ScheduleStart         string                 `json:"scheduleStart"`
	ScheduleEnd           *string                `json:"scheduleEnd"`
	CreatedAt             *time.Time             `json:"createdAt"`
	UpdatedAt             *time.Time             `json:"updatedAt"`
	BiddingStrategy       BiddingStrategy        `json:"biddingStrategy,omitempty"`
	CPIBids               []CPIBid               `json:"cpiBids,omitempty"`
	SourceBids            []SourceBid            `json:"sourceBids,omitempty"`
	ROASTypes             []ROASType             `json:"roasTypes,omitempty"`
	PostInstallWindow     PostInstallWindow      `json:"postInstallWindow,omitempty"`
	ROASBids              []ROASBid              `json:"roasBids,omitempty"`
	RetentionBids         []RetentionBid         `json:"retentionBids,omitempty"`
	EventOptimizationBids []EventOptimizationBid `json:"eventOptimizationBids,omitempty"`
	EventOptimizationType EventOptimizationType  `json:"eventOptimizationType,omitempty"`
	SDKEventName          *SDKEventName          `json:"sdkEventName"`
	Budget                *CampaignBudget        `json:"budget,omitempty"`
	AutoStart             AutoStart              `json:"autoStart,omitempty"`
	Raw                   json.RawMessage        `json:"-"`
}

type ListCampaignsRequest struct {
	Enabled *bool
}

type GetCampaignRequest struct {
	IncludeFields []CampaignIncludeField
}

type CampaignCreateRequest interface {
	isCampaignCreateRequest()
}

type CampaignCreateBase struct {
	Name                string          `json:"name"`
	BillingType         BillingType     `json:"billingType,omitempty"`
	BiddingStrategy     BiddingStrategy `json:"biddingStrategy,omitempty"`
	AttributionClickURL *string         `json:"attributionClickUrl,omitempty"`
	AttributionStartURL *string         `json:"attributionStartUrl,omitempty"`
	ScheduleStart       *string         `json:"scheduleStart,omitempty"`
	ScheduleEnd         *string         `json:"scheduleEnd,omitempty"`
}

type CreateInstallsCampaignRequest struct {
	CampaignCreateBase
	Goal CampaignGoal `json:"goal"`
}

func (CreateInstallsCampaignRequest) isCampaignCreateRequest() {}

type CreateRetentionCampaignRequest struct {
	CampaignCreateBase
	Goal CampaignGoal `json:"goal"`
}

func (CreateRetentionCampaignRequest) isCampaignCreateRequest() {}

type CreateROASCampaignRequest struct {
	CampaignCreateBase
	Goal              CampaignGoal      `json:"goal"`
	ROASTypes         []ROASType        `json:"roasTypes,omitempty"`
	PostInstallWindow PostInstallWindow `json:"postInstallWindow,omitempty"`
}

func (CreateROASCampaignRequest) isCampaignCreateRequest() {}

type CreateCreativeTestingCampaignRequest struct {
	CampaignCreateBase
	Goal CampaignGoal `json:"goal"`
}

func (CreateCreativeTestingCampaignRequest) isCampaignCreateRequest() {}

type CreateEventOptimizationCampaignRequest struct {
	CampaignCreateBase
	Goal                  CampaignGoal          `json:"goal"`
	EventOptimizationType EventOptimizationType `json:"eventOptimizationType,omitempty"`
	SDKEventName          *NullableString       `json:"sdkEventName,omitempty"`
}

func (CreateEventOptimizationCampaignRequest) isCampaignCreateRequest() {}

type UpdateCampaignRequest struct {
	Name                *string         `json:"name,omitempty"`
	Enabled             *bool           `json:"enabled,omitempty"`
	AttributionClickURL *NullableString `json:"attributionClickUrl,omitempty"`
	AttributionStartURL *NullableString `json:"attributionStartUrl,omitempty"`
	ScheduleStart       *string         `json:"scheduleStart,omitempty"`
	ScheduleEnd         *NullableString `json:"scheduleEnd,omitempty"`
	AutoStart           *AutoStart      `json:"autoStart,omitempty"`
}

type AssignedCreativePack struct {
	ID  string          `json:"id"`
	Raw json.RawMessage `json:"-"`
}

type CampaignsWorkflow interface {
	ListCampaigns(context.Context, string, ListCampaignsRequest, ...socialhub.CallOption) (Page[Campaign], error)
	CreateCampaign(context.Context, string, CampaignCreateRequest, ...socialhub.CallOption) (*Campaign, error)
	GetCampaign(context.Context, string, string, GetCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	DeleteCampaign(context.Context, string, string, ...socialhub.CallOption) error
	ListAssignedCreativePacks(context.Context, string, string, ...socialhub.CallOption) (Page[AssignedCreativePack], error)
	AssignCreativePack(context.Context, string, string, string, ...socialhub.CallOption) (*AssignedCreativePack, error)
	UnassignCreativePack(context.Context, string, string, string, ...socialhub.CallOption) error
	GetTargeting(context.Context, string, string, ...socialhub.CallOption) (*Targeting, error)
	UpdateTargeting(context.Context, string, string, Targeting, ...socialhub.CallOption) (*Targeting, error)
	GetCampaignBudget(context.Context, string, string, ...socialhub.CallOption) (*CampaignBudget, error)
	UpdateCampaignBudget(context.Context, string, string, CampaignBudgetUpdate, ...socialhub.CallOption) (*CampaignBudget, error)
	ListSDKEventNames(context.Context, string, ...socialhub.CallOption) (Page[SDKEventNamesInfo], error)
}

func (client *Client) ListCampaigns(ctx context.Context, campaignSetID string, input ListCampaignsRequest, options ...socialhub.CallOption) (Page[Campaign], error) {
	appPath, err := client.appPath("campaign_list", campaignSetID)
	if err != nil {
		return Page[Campaign]{}, err
	}
	query := make(url.Values)
	if input.Enabled != nil {
		query.Set("filter[enabled]", formatBool(*input.Enabled))
	}
	var page Page[Campaign]
	if err := client.getJSON(ctx, "campaign_list", appPath+"/campaigns", query, &page, options...); err != nil {
		return Page[Campaign]{}, err
	}
	if !validPage(page, 0, validCampaign) {
		return Page[Campaign]{}, platformContractError("campaign_list", "Unity returned an invalid campaign page")
	}
	return page, nil
}

func (client *Client) CreateCampaign(ctx context.Context, campaignSetID string, input CampaignCreateRequest, options ...socialhub.CallOption) (*Campaign, error) {
	appPath, err := client.appPath("campaign_create", campaignSetID)
	if err != nil {
		return nil, err
	}
	if err := validateCreateCampaign(input); err != nil {
		return nil, err
	}
	var campaign Campaign
	if err := client.postJSON(ctx, "campaign_create", appPath+"/campaigns", input, &campaign, options...); err != nil {
		return nil, err
	}
	if !validCampaign(campaign) {
		return nil, platformContractError("campaign_create", "Unity returned an invalid campaign")
	}
	return &campaign, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignSetID, campaignID string, input GetCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	path, err := client.campaignPath("campaign_get", campaignSetID, campaignID)
	if err != nil {
		return nil, err
	}
	query := make(url.Values)
	seen := make(map[CampaignIncludeField]struct{}, len(input.IncludeFields))
	for _, field := range input.IncludeFields {
		if !validIncludeField(field) {
			return nil, invalidArgument("campaign_get", "includeFields contains an unsupported value")
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		query.Add("includeFields", string(field))
	}
	var campaign Campaign
	if err := client.getJSON(ctx, "campaign_get", path, query, &campaign, options...); err != nil {
		return nil, err
	}
	if !validCampaign(campaign) || campaign.ID != campaignID {
		return nil, platformContractError("campaign_get", "Unity returned a campaign that does not match the requested ID")
	}
	return &campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignSetID, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	path, err := client.campaignPath("campaign_update", campaignSetID, campaignID)
	if err != nil {
		return nil, err
	}
	if err := validateUpdateCampaign(input); err != nil {
		return nil, err
	}
	var campaign Campaign
	if err := client.patchJSON(ctx, "campaign_update", path, input, &campaign, options...); err != nil {
		return nil, err
	}
	if !validCampaign(campaign) || campaign.ID != campaignID {
		return nil, platformContractError("campaign_update", "Unity returned a campaign that does not match the requested ID")
	}
	return &campaign, nil
}

func (client *Client) DeleteCampaign(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) error {
	path, err := client.campaignPath("campaign_delete", campaignSetID, campaignID)
	if err != nil {
		return err
	}
	return client.deleteJSON(ctx, "campaign_delete", path, options...)
}

func (client *Client) ListAssignedCreativePacks(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (Page[AssignedCreativePack], error) {
	path, err := client.campaignPath("assigned_creative_pack_list", campaignSetID, campaignID)
	if err != nil {
		return Page[AssignedCreativePack]{}, err
	}
	var page Page[AssignedCreativePack]
	if err := client.getJSON(ctx, "assigned_creative_pack_list", path+"/assigned-creative-packs", nil, &page, options...); err != nil {
		return Page[AssignedCreativePack]{}, err
	}
	if !validPage(page, 0, func(item AssignedCreativePack) bool { return validMongoID(item.ID) }) {
		return Page[AssignedCreativePack]{}, platformContractError("assigned_creative_pack_list", "Unity returned an invalid creative pack assignment page")
	}
	return page, nil
}

func (client *Client) AssignCreativePack(ctx context.Context, campaignSetID, campaignID, creativePackID string, options ...socialhub.CallOption) (*AssignedCreativePack, error) {
	path, err := client.campaignPath("creative_pack_assign", campaignSetID, campaignID)
	if err != nil {
		return nil, err
	}
	if !validMongoID(creativePackID) {
		return nil, invalidArgument("creative_pack_assign", "creative pack ID must be a 24-character hexadecimal ID")
	}
	var assigned AssignedCreativePack
	if err := client.postJSON(ctx, "creative_pack_assign", path+"/assigned-creative-packs", struct {
		ID string `json:"id"`
	}{ID: creativePackID}, &assigned, options...); err != nil {
		return nil, err
	}
	if assigned.ID != creativePackID {
		return nil, platformContractError("creative_pack_assign", "Unity returned an assignment that does not match the requested creative pack ID")
	}
	return &assigned, nil
}

func (client *Client) UnassignCreativePack(ctx context.Context, campaignSetID, campaignID, assignedCreativePackID string, options ...socialhub.CallOption) error {
	path, err := client.campaignPath("creative_pack_unassign", campaignSetID, campaignID)
	if err != nil {
		return err
	}
	if !validMongoID(assignedCreativePackID) {
		return invalidArgument("creative_pack_unassign", "assigned creative pack ID must be a 24-character hexadecimal ID")
	}
	return client.deleteJSON(ctx, "creative_pack_unassign", path+"/assigned-creative-packs/"+assignedCreativePackID, options...)
}

func validateCreateCampaign(input CampaignCreateRequest) error {
	var base CampaignCreateBase
	var goal CampaignGoal
	switch typed := input.(type) {
	case CreateInstallsCampaignRequest:
		base, goal = typed.CampaignCreateBase, typed.Goal
		if goal != CampaignGoalInstalls {
			return invalidArgument("campaign_create", "installs campaign requires goal=installs")
		}
	case *CreateInstallsCampaignRequest:
		if typed == nil {
			return invalidArgument("campaign_create", "campaign request is required")
		}
		return validateCreateCampaign(*typed)
	case CreateRetentionCampaignRequest:
		base, goal = typed.CampaignCreateBase, typed.Goal
		if goal != CampaignGoalRetention || base.BiddingStrategy != "" {
			return invalidArgument("campaign_create", "retention campaign requires goal=retention and does not accept biddingStrategy")
		}
	case *CreateRetentionCampaignRequest:
		if typed == nil {
			return invalidArgument("campaign_create", "campaign request is required")
		}
		return validateCreateCampaign(*typed)
	case CreateROASCampaignRequest:
		base, goal = typed.CampaignCreateBase, typed.Goal
		if goal != CampaignGoalROAS || base.BiddingStrategy != "" || !validROASTypes(typed.ROASTypes) || typed.PostInstallWindow != "" && !validCreatePostInstallWindow(typed.PostInstallWindow) {
			return invalidArgument("campaign_create", "ROAS campaign goal, ROAS types, or post-install window is invalid")
		}
	case *CreateROASCampaignRequest:
		if typed == nil {
			return invalidArgument("campaign_create", "campaign request is required")
		}
		return validateCreateCampaign(*typed)
	case CreateCreativeTestingCampaignRequest:
		base, goal = typed.CampaignCreateBase, typed.Goal
		if goal != CampaignGoalCreativeTesting {
			return invalidArgument("campaign_create", "creative testing campaign requires goal=creativeTesting")
		}
	case *CreateCreativeTestingCampaignRequest:
		if typed == nil {
			return invalidArgument("campaign_create", "campaign request is required")
		}
		return validateCreateCampaign(*typed)
	case CreateEventOptimizationCampaignRequest:
		base, goal = typed.CampaignCreateBase, typed.Goal
		if goal != CampaignGoalEventOptimization || typed.EventOptimizationType != "" && !validText(string(typed.EventOptimizationType), 255) ||
			!validNullableText(typed.SDKEventName, 1, 255) {
			return invalidArgument("campaign_create", "event optimization campaign fields are invalid")
		}
	case *CreateEventOptimizationCampaignRequest:
		if typed == nil {
			return invalidArgument("campaign_create", "campaign request is required")
		}
		return validateCreateCampaign(*typed)
	default:
		return invalidArgument("campaign_create", "campaign request type is unsupported")
	}
	_ = goal
	return validateCampaignCreateBase(base)
}

func validateCampaignCreateBase(base CampaignCreateBase) error {
	if !validText(base.Name, 255) || base.BillingType != "" && !validBillingType(base.BillingType) ||
		base.BiddingStrategy != "" && !validBiddingStrategy(base.BiddingStrategy) ||
		base.AttributionClickURL != nil && !validHTTPSURL(*base.AttributionClickURL) ||
		base.AttributionStartURL != nil && !validHTTPSURL(*base.AttributionStartURL) ||
		!validOptionalDate(base.ScheduleStart) || !validOptionalDate(base.ScheduleEnd) {
		return invalidArgument("campaign_create", "campaign name, billing, attribution URL, or schedule is invalid")
	}
	return nil
}

func validateUpdateCampaign(input UpdateCampaignRequest) error {
	if input.Name == nil && input.Enabled == nil && input.AttributionClickURL == nil && input.AttributionStartURL == nil &&
		input.ScheduleStart == nil && input.ScheduleEnd == nil && input.AutoStart == nil {
		return invalidArgument("campaign_update", "at least one campaign field is required")
	}
	if input.Name != nil && !validText(*input.Name, 255) || !validNullableHTTPSURL(input.AttributionClickURL) ||
		!validNullableHTTPSURL(input.AttributionStartURL) || !validOptionalDate(input.ScheduleStart) {
		return invalidArgument("campaign_update", "campaign name, attribution URL, or schedule start is invalid")
	}
	if input.ScheduleEnd != nil && input.ScheduleEnd.Value != nil && !validDate(*input.ScheduleEnd.Value) {
		return invalidArgument("campaign_update", "campaign schedule end is invalid")
	}
	if input.AutoStart != nil && *input.AutoStart != AutoStartEnabled && *input.AutoStart != AutoStartDisabled {
		return invalidArgument("campaign_update", "autoStart can only be enabled or disabled")
	}
	return nil
}

func validCampaign(campaign Campaign) bool {
	return validMongoID(campaign.ID) && validCampaignGoal(campaign.Goal)
}

func validCampaignGoal(value CampaignGoal) bool {
	switch value {
	case CampaignGoalInstalls, CampaignGoalRetention, CampaignGoalROAS, CampaignGoalCreativeTesting, CampaignGoalEventOptimization:
		return true
	default:
		return false
	}
}

func validBillingType(value BillingType) bool { return value == BillingCPI || value == BillingCPM }
func validBiddingStrategy(value BiddingStrategy) bool {
	return value == BiddingManual || value == BiddingAutomated
}

func validROASTypes(values []ROASType) bool {
	if len(values) == 0 {
		return true
	}
	seen := make(map[ROASType]struct{}, len(values))
	for _, value := range values {
		if value != ROASTypeIAP && value != ROASTypeAdRevenue {
			return false
		}
		seen[value] = struct{}{}
	}
	return len(seen) == len(values)
}

func validCreatePostInstallWindow(value PostInstallWindow) bool {
	return value == PostInstallD0 || value == PostInstallD7 || value == PostInstallD28
}

func validIncludeField(value CampaignIncludeField) bool {
	switch value {
	case IncludeCPIBids, IncludeSourceBids, IncludeROASBids, IncludeRetentionBids, IncludeEventOptimizationBids, IncludeBudget:
		return true
	default:
		return false
	}
}

func (campaign *Campaign) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*campaignAlias)(campaign), &campaign.Raw)
}

func (assigned *AssignedCreativePack) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*assignedCreativePackAlias)(assigned), &assigned.Raw)
}

type campaignAlias Campaign
type assignedCreativePackAlias AssignedCreativePack
