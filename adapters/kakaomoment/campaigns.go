package kakaomoment

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type campaignListResponse struct {
	Content []Campaign `json:"content"`
}

type campaignCreateWire struct {
	Name              string           `json:"name,omitempty"`
	CampaignTypeGoal  CampaignTypeGoal `json:"campaignTypeGoal"`
	Objective         *Objective       `json:"objective,omitempty"`
	DailyBudgetAmount *int64           `json:"dailyBudgetAmount,omitempty"`
	TrackID           string           `json:"trackId,omitempty"`
	KCLID             *bool            `json:"kclid,omitempty"`
}

type campaignUpdateWire struct {
	ID      int64   `json:"id"`
	Name    *string `json:"name,omitempty"`
	TrackID *string `json:"trackId,omitempty"`
	KCLID   *bool   `json:"kclid,omitempty"`
}

type campaignBudgetWire struct {
	ID                int64  `json:"id"`
	DailyBudgetAmount *int64 `json:"dailyBudgetAmount"`
}

type configWire struct {
	ID     int64        `json:"id"`
	Config ConfigStatus `json:"config"`
}

func (client *Client) ListCampaigns(ctx context.Context, config ConfigStatus, options ...socialhub.CallOption) ([]Campaign, error) {
	const operation = "campaign_list"
	if config != "" && !validConfig(config, true) {
		return nil, invalidArgument(operation, "campaign config filter is invalid")
	}
	query := make(url.Values)
	if config != "" {
		query.Set("config", string(config))
	}
	var response campaignListResponse
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"campaigns", query, nil, &response, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if response.Content == nil {
		return nil, platformContractError(operation, "Kakao Moment campaign response omitted content")
	}
	for index := range response.Content {
		if err := client.validateCampaign(operation, &response.Content[index], false); err != nil {
			return nil, err
		}
	}
	return response.Content, nil
}

func (client *Client) GetCampaign(ctx context.Context, id int64, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	var campaign Campaign
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"campaigns/"+formatID(id), nil, nil, &campaign, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &campaign, true); err != nil {
		return nil, err
	}
	if campaign.ID != id {
		return nil, platformContractError(operation, "Kakao Moment returned a different campaign")
	}
	return &campaign, nil
}

func (client *Client) CreateCampaignThenPause(ctx context.Context, input CampaignCreate, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create_then_pause"
	if !validCampaignCreate(input) {
		return nil, invalidArgument(operation, "campaign type, goal, name, objective, tracking ID, or budget is invalid")
	}
	wire := campaignCreateWire{
		Name: input.Name, CampaignTypeGoal: input.CampaignTypeGoal, Objective: input.Objective,
		DailyBudgetAmount: input.DailyBudgetAmount, TrackID: input.TrackID, KCLID: input.KCLID,
	}
	var campaign Campaign
	requestID, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPost,
		"campaigns", nil, wire, &campaign, true, options...,
	)
	if err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &campaign, true); err != nil {
		return nil, outcomeUnknownError(operation, err, requestID)
	}
	if campaign.CampaignTypeGoal == nil || campaign.CampaignTypeGoal.CampaignType != input.CampaignTypeGoal.CampaignType ||
		campaign.CampaignTypeGoal.Goal != input.CampaignTypeGoal.Goal {
		return &campaign, outcomeUnknownError(operation, platformContractError(operation, "created Campaign type or goal did not match the request"), requestID)
	}
	if err := client.SetCampaignConfig(ctx, campaign.ID, ConfigOff, options...); err != nil {
		return &campaign, reconciliationError(operation, campaign.ID, err)
	}
	campaign.Config = ConfigOff
	return &campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, input CampaignUpdate, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validCampaignUpdate(input) {
		return nil, invalidArgument(operation, "campaign update is invalid")
	}
	wire := campaignUpdateWire{ID: input.ID, Name: input.Name, TrackID: input.TrackID, KCLID: input.KCLID}
	var campaign Campaign
	requestID, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPut,
		"campaigns", nil, wire, &campaign, true, options...,
	)
	if err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &campaign, true); err != nil || campaign.ID != input.ID {
		if err == nil {
			err = platformContractError(operation, "Kakao Moment returned a different campaign")
		}
		return &campaign, outcomeUnknownError(operation, err, requestID)
	}
	return &campaign, nil
}

func (client *Client) SetCampaignDailyBudget(ctx context.Context, id int64, amount *int64, options ...socialhub.CallOption) error {
	const operation = "campaign_set_daily_budget"
	if id <= 0 || amount != nil && (*amount < 50_000 || *amount > 1_000_000_000 || *amount%10 != 0) {
		return invalidArgument(operation, "campaign ID or daily budget is invalid; a non-null budget must be 50,000-1,000,000,000 KRW in multiples of 10")
	}
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPut,
		"campaigns/dailyBudgetAmount", nil, campaignBudgetWire{ID: id, DailyBudgetAmount: amount}, nil, true, options...,
	)
	return err
}

func (client *Client) SetCampaignConfig(ctx context.Context, id int64, config ConfigStatus, options ...socialhub.CallOption) error {
	const operation = "campaign_set_config"
	if id <= 0 || !validConfig(config, false) {
		return invalidArgument(operation, "campaign ID must be positive and config must be ON or OFF")
	}
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPut,
		"campaigns/onOff", nil, configWire{ID: id, Config: config}, nil, true, options...,
	)
	return err
}

func (client *Client) DeleteCampaign(ctx context.Context, id int64, options ...socialhub.CallOption) error {
	const operation = "campaign_delete"
	if id <= 0 {
		return invalidArgument(operation, "campaign ID must be positive")
	}
	campaign, err := client.GetCampaign(ctx, id, options...)
	if err != nil {
		return withOperation(err, operation)
	}
	if campaign.Config != ConfigOff {
		return conflict(operation, "campaign must be OFF before guarded deletion")
	}
	_, err = client.doJSON(
		ctx, operation, []string{ScopeManagement, ScopeDelete}, http.MethodDelete,
		"campaigns/"+formatID(id), nil, nil, nil, true, options...,
	)
	return err
}

func (client *Client) validateCampaign(operation string, campaign *Campaign, detailed bool) error {
	if campaign == nil || campaign.ID <= 0 || !validText(campaign.Name, 1024) || !validConfig(campaign.Config, true) {
		return platformContractError(operation, "Kakao Moment returned an invalid campaign")
	}
	if !detailed {
		return nil
	}
	if campaign.AdAccountID != client.adAccountID || campaign.CampaignTypeGoal == nil ||
		!validEnumToken(campaign.CampaignTypeGoal.CampaignType) || !validEnumToken(campaign.CampaignTypeGoal.Goal) ||
		campaign.DailyBudgetAmount != nil && *campaign.DailyBudgetAmount < 0 {
		return platformContractError(operation, "Kakao Moment returned invalid Campaign detail")
	}
	return nil
}
