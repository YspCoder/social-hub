package ads

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

type createCampaignData struct {
	Name                         string            `json:"name"`
	ConfiguredStatus             ConfiguredStatus  `json:"configured_status"`
	Objective                    CampaignObjective `json:"objective"`
	FundingInstrumentID          string            `json:"funding_instrument_id"`
	IsCampaignBudgetOptimization bool              `json:"is_campaign_budget_optimization"`
	GoalType                     GoalType          `json:"goal_type,omitempty"`
	GoalValue                    *int64            `json:"goal_value,omitempty"`
	SpendCap                     *int64            `json:"spend_cap,omitempty"`
	StartTime                    string            `json:"start_time,omitempty"`
	EndTime                      string            `json:"end_time,omitempty"`
	BidStrategy                  BidStrategy       `json:"bid_strategy,omitempty"`
	BidType                      BidType           `json:"bid_type,omitempty"`
	BidValue                     *int64            `json:"bid_value,omitempty"`
	ConversionPixelID            string            `json:"conversion_pixel_id,omitempty"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[Campaign], error) {
	const operation = "campaigns_list"
	if !validList(input) {
		return socialhub.Page[Campaign]{}, invalidArgument(operation, "pagination is invalid")
	}
	path := client.accountPath("campaigns")
	var response listResponse[Campaign]
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	for index := range response.Data {
		if err := client.validateCampaign(operation, &response.Data[index], "", "", false); err != nil {
			return socialhub.Page[Campaign]{}, err
		}
	}
	cursor, err := client.pageCursor(operation, path, response.Pagination.NextURL)
	if err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	return page(response.Data, cursor), nil
}

func (client *Client) GetCampaign(ctx context.Context, id string, options ...socialhub.CallOption) (*Campaign, error) {
	return client.getCampaign(ctx, "campaign_get", id, options...)
}

func (client *Client) getCampaign(ctx context.Context, operation, id string, options ...socialhub.CallOption) (*Campaign, error) {
	if !validResourceID(id) {
		return nil, invalidArgument(operation, "Campaign ID must be numeric")
	}
	var response singleResponse[Campaign]
	if _, err := client.getJSON(ctx, operation, "/campaigns/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, id, "", false); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if err := validateCreateCampaign(input); err != nil {
		return nil, invalidArgument(operation, err.Error())
	}
	if _, err := client.getFundingInstrument(ctx, operation, input.FundingInstrumentID, options...); err != nil {
		return nil, err
	}
	data := createCampaignData{
		Name: input.Name, ConfiguredStatus: StatusPaused, Objective: input.Objective,
		FundingInstrumentID: input.FundingInstrumentID, IsCampaignBudgetOptimization: input.IsCampaignBudgetOptimization,
		GoalType: input.GoalType, GoalValue: input.GoalValue, SpendCap: input.SpendCap,
		BidStrategy: input.BidStrategy, BidType: input.BidType, BidValue: input.BidValue,
		ConversionPixelID: input.ConversionPixelID,
	}
	if input.StartTime != nil {
		data.StartTime = input.StartTime.UTC().Format(time.RFC3339)
	}
	if input.EndTime != nil {
		data.EndTime = input.EndTime.UTC().Format(time.RFC3339)
	}
	var response singleResponse[Campaign]
	path := client.accountPath("campaigns")
	if _, err := client.writeJSON(ctx, operation, http.MethodPost, path, nil, struct {
		Data createCampaignData `json:"data"`
	}{Data: data}, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, "", input.FundingInstrumentID, true); err != nil {
		return nil, err
	}
	if response.Data.ConfiguredStatus != StatusPaused {
		return nil, platformContractError(operation, "Reddit did not create the Campaign in PAUSED state")
	}
	if input.IsCampaignBudgetOptimization && !campaignCBO(response.Data) {
		return nil, platformContractError(operation, "Reddit did not preserve Campaign budget optimization")
	}
	return &response.Data, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, id string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validResourceID(id) || input.Name == nil && input.Status == nil ||
		input.Name != nil && !validText(*input.Name, 500) || input.Status != nil && !validMutationStatus(*input.Status) {
		return nil, invalidArgument(operation, "Campaign ID or update fields are invalid")
	}
	current, err := client.getCampaign(ctx, operation, id, options...)
	if err != nil {
		return nil, err
	}
	data := make(map[string]any, 2)
	if input.Name != nil {
		data["name"] = *input.Name
	}
	if input.Status != nil {
		data["configured_status"] = *input.Status
	}
	var response singleResponse[Campaign]
	if _, err := client.writeJSON(ctx, operation, http.MethodPatch, "/campaigns/"+id, nil, struct {
		Data map[string]any `json:"data"`
	}{Data: data}, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, id, current.FundingInstrumentID, true); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) validateCampaign(operation string, value *Campaign, expectedID, expectedFundingID string, strictFunding bool) error {
	if !validResourceID(value.ID) || expectedID != "" && value.ID != expectedID {
		return platformContractError(operation, "Reddit returned a missing or mismatched Campaign ID")
	}
	if value.AdAccountID != "" && value.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Reddit returned a Campaign owned by another Ad Account")
	}
	if value.AdAccountID == "" {
		value.AdAccountID = client.adAccountID
	}
	if value.FundingInstrumentID != "" && !validResourceID(value.FundingInstrumentID) {
		return platformContractError(operation, "Reddit returned an invalid Campaign Funding Instrument")
	}
	if expectedFundingID != "" && value.FundingInstrumentID != expectedFundingID {
		if strictFunding || value.FundingInstrumentID != "" {
			return platformContractError(operation, "Reddit returned a Campaign for another Funding Instrument")
		}
		value.FundingInstrumentID = expectedFundingID
	}
	return nil
}

func campaignCBO(value Campaign) bool {
	return value.IsCampaignBudgetOptimization != nil && *value.IsCampaignBudgetOptimization
}

func validateCreateCampaign(input CreateCampaignRequest) error {
	if !validResourceID(input.FundingInstrumentID) || !validText(input.Name, 500) || !validObjective(input.Objective) ||
		input.GoalValue != nil && *input.GoalValue <= 0 || input.SpendCap != nil && *input.SpendCap <= 0 ||
		input.BidValue != nil && *input.BidValue <= 0 ||
		input.StartTime != nil && input.StartTime.IsZero() || input.EndTime != nil && input.EndTime.IsZero() ||
		input.StartTime != nil && input.EndTime != nil && !input.EndTime.After(*input.StartTime) {
		return fmt.Errorf("Funding Instrument, name, objective, budget, bid, or schedule is invalid")
	}
	if input.IsCampaignBudgetOptimization {
		if !validGoalType(input.GoalType) || input.GoalValue == nil || *input.GoalValue <= 0 || input.StartTime == nil ||
			!validCampaignBidStrategy(input.BidStrategy) || !validBidType(input.BidType) || !validPixelID(input.ConversionPixelID) {
			return fmt.Errorf("CBO Campaigns require goal, schedule, bid, and conversion_pixel_id")
		}
		if input.GoalType == GoalLifetimeSpend && input.EndTime == nil {
			return fmt.Errorf("lifetime CBO Campaigns require an end time")
		}
		if input.BidStrategy == BidStrategyTargetCPX && input.BidValue == nil ||
			input.BidStrategy == BidStrategyBidless && input.BidValue != nil {
			return fmt.Errorf("CBO Campaign bid strategy and bid value are incompatible")
		}
	} else if input.GoalType != "" || input.GoalValue != nil || input.BidStrategy != "" || input.BidType != "" || input.BidValue != nil ||
		input.ConversionPixelID != "" || input.StartTime != nil || input.EndTime != nil {
		return fmt.Errorf("non-CBO Campaigns cannot set Campaign-level goal, bid, conversion_pixel_id, or schedule")
	}
	return nil
}
