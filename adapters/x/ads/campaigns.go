package ads

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[Campaign], error) {
	const operation = "campaigns_list"
	if !validList(input.Cursor, input.Count) {
		return socialhub.Page[Campaign]{}, invalidArgument(operation, "cursor or count is invalid")
	}
	var response listResponse[Campaign]
	if _, err := client.get(ctx, client.resourcePath("campaigns"), listQuery(input.Cursor, input.Count), &response, options...); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	for index := range response.Data {
		if err := client.validateCampaign(operation, &response.Data[index], "", ""); err != nil {
			return socialhub.Page[Campaign]{}, err
		}
	}
	return cursorPage(operation, response.Data, response.NextCursor)
}

func (client *Client) GetCampaign(ctx context.Context, id string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validAdsID(id) {
		return nil, invalidArgument(operation, "Campaign ID must be base36")
	}
	var response singleResponse[Campaign]
	if _, err := client.get(ctx, client.resourcePath("campaigns")+"/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, id, ""); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validAdsID(input.FundingInstrumentID) || !validText(input.Name, 255) || input.DailyBudgetAmountLocalMicro <= 0 ||
		input.TotalBudgetAmountLocalMicro != nil && (*input.TotalBudgetAmountLocalMicro <= 0 || input.DailyBudgetAmountLocalMicro > *input.TotalBudgetAmountLocalMicro) {
		return nil, invalidArgument(operation, "Funding Instrument, name, or budget is invalid")
	}
	if _, err := client.getFundingInstrument(ctx, operation, input.FundingInstrumentID, options...); err != nil {
		return nil, err
	}
	query := url.Values{
		"funding_instrument_id": {input.FundingInstrumentID}, "name": {input.Name},
		"daily_budget_amount_local_micro": {formatInt64(input.DailyBudgetAmountLocalMicro)},
		"entity_status":                   {string(StatusPaused)}, "budget_optimization": {"LINE_ITEM"},
	}
	if input.TotalBudgetAmountLocalMicro != nil {
		query.Set("total_budget_amount_local_micro", formatInt64(*input.TotalBudgetAmountLocalMicro))
	}
	var response singleResponse[Campaign]
	if _, err := client.write(ctx, http.MethodPost, client.resourcePath("campaigns"), query, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, "", input.FundingInstrumentID); err != nil {
		return nil, err
	}
	if response.Data.EntityStatus != StatusPaused {
		return nil, platformContractError(operation, "X did not create the Campaign in PAUSED state")
	}
	return &response.Data, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, id string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validAdsID(id) || input.Name != nil && !validText(*input.Name, 255) ||
		input.Status != nil && !validMutationStatus(*input.Status) ||
		input.DailyBudgetAmountLocalMicro != nil && *input.DailyBudgetAmountLocalMicro <= 0 ||
		input.TotalBudgetAmountLocalMicro != nil && *input.TotalBudgetAmountLocalMicro <= 0 ||
		input.Name == nil && input.Status == nil && input.DailyBudgetAmountLocalMicro == nil && input.TotalBudgetAmountLocalMicro == nil {
		return nil, invalidArgument(operation, "Campaign ID or update fields are invalid")
	}
	current, err := client.GetCampaign(ctx, id, options...)
	if err != nil {
		return nil, err
	}
	daily, total := current.DailyBudgetAmountLocalMicro, current.TotalBudgetAmountLocalMicro
	if input.DailyBudgetAmountLocalMicro != nil {
		daily = input.DailyBudgetAmountLocalMicro
	}
	if input.TotalBudgetAmountLocalMicro != nil {
		total = input.TotalBudgetAmountLocalMicro
	}
	if daily != nil && total != nil && *daily > *total {
		return nil, invalidArgument(operation, "daily Campaign budget must not exceed total budget")
	}
	query := make(url.Values)
	if input.Name != nil {
		query.Set("name", *input.Name)
	}
	if input.Status != nil {
		query.Set("entity_status", string(*input.Status))
	}
	if input.DailyBudgetAmountLocalMicro != nil {
		query.Set("daily_budget_amount_local_micro", formatInt64(*input.DailyBudgetAmountLocalMicro))
	}
	if input.TotalBudgetAmountLocalMicro != nil {
		query.Set("total_budget_amount_local_micro", formatInt64(*input.TotalBudgetAmountLocalMicro))
	}
	var response singleResponse[Campaign]
	if _, err := client.write(ctx, http.MethodPut, client.resourcePath("campaigns")+"/"+id, query, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response.Data, id, current.FundingInstrumentID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) validateCampaign(operation string, value *Campaign, expectedID, expectedFundingID string) error {
	if !validAdsID(value.ID) || expectedID != "" && value.ID != expectedID {
		return platformContractError(operation, "X returned a missing or mismatched Campaign ID")
	}
	if value.AccountID != "" && value.AccountID != client.adsAccountID {
		return platformContractError(operation, "X returned a Campaign owned by another Ads Account")
	}
	if value.AccountID == "" {
		value.AccountID = client.adsAccountID
	}
	if value.FundingInstrumentID != "" && !validAdsID(value.FundingInstrumentID) ||
		expectedFundingID != "" && value.FundingInstrumentID != expectedFundingID {
		return platformContractError(operation, "X returned a Campaign for another Funding Instrument")
	}
	return nil
}

func cursorPage[T any](operation string, items []T, cursor *string) (socialhub.Page[T], error) {
	if cursor != nil && !validOpaque(*cursor, 16384) {
		return socialhub.Page[T]{}, platformContractError(operation, "X returned an invalid pagination cursor")
	}
	return socialhub.Page[T]{Items: items, NextCursor: cursor, HasMore: cursor != nil}, nil
}
