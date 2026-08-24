package ads

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListLineItems(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[LineItem], error) {
	const operation = "line_items_list"
	if !validList(input.Cursor, input.Count) {
		return socialhub.Page[LineItem]{}, invalidArgument(operation, "cursor or count is invalid")
	}
	var response listResponse[LineItem]
	if _, err := client.get(ctx, client.resourcePath("line_items"), listQuery(input.Cursor, input.Count), &response, options...); err != nil {
		return socialhub.Page[LineItem]{}, err
	}
	for index := range response.Data {
		if err := client.validateLineItem(operation, &response.Data[index], "", ""); err != nil {
			return socialhub.Page[LineItem]{}, err
		}
	}
	return cursorPage(operation, response.Data, response.NextCursor)
}

func (client *Client) GetLineItem(ctx context.Context, id string, options ...socialhub.CallOption) (*LineItem, error) {
	const operation = "line_item_get"
	if !validAdsID(id) {
		return nil, invalidArgument(operation, "Line Item ID must be base36")
	}
	var response singleResponse[LineItem]
	if _, err := client.get(ctx, client.resourcePath("line_items")+"/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateLineItem(operation, &response.Data, id, ""); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) CreateLineItem(ctx context.Context, input CreateLineItemRequest, options ...socialhub.CallOption) (*LineItem, error) {
	const operation = "line_item_create"
	if !validCreateLineItem(input) {
		return nil, invalidArgument(operation, "Campaign, objective, product, placement, bid, budget, or schedule is invalid")
	}
	if _, err := client.GetCampaign(ctx, input.CampaignID, options...); err != nil {
		return nil, err
	}
	query := url.Values{
		"campaign_id": {input.CampaignID}, "objective": {string(input.Objective)},
		"product_type": {string(input.ProductType)}, "placements": {joinPlacements(input.Placements)},
		"bid_strategy": {string(input.BidStrategy)}, "entity_status": {string(StatusPaused)},
		"start_time": {input.StartTime.UTC().Format(time.RFC3339)},
	}
	if input.Name != "" {
		query.Set("name", input.Name)
	}
	setOptionalMicro(query, "bid_amount_local_micro", input.BidAmountLocalMicro)
	setOptionalMicro(query, "daily_budget_amount_local_micro", input.DailyBudgetAmountLocalMicro)
	setOptionalMicro(query, "total_budget_amount_local_micro", input.TotalBudgetAmountLocalMicro)
	if input.EndTime != nil {
		query.Set("end_time", input.EndTime.UTC().Format(time.RFC3339))
	}
	var response singleResponse[LineItem]
	if _, err := client.write(ctx, http.MethodPost, client.resourcePath("line_items"), query, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateLineItem(operation, &response.Data, "", input.CampaignID); err != nil {
		return nil, err
	}
	if response.Data.EntityStatus != StatusPaused {
		return nil, platformContractError(operation, "X did not create the Line Item in PAUSED state")
	}
	return &response.Data, nil
}

func (client *Client) UpdateLineItem(ctx context.Context, id string, input UpdateLineItemRequest, options ...socialhub.CallOption) (*LineItem, error) {
	const operation = "line_item_update"
	if !validAdsID(id) || input.Name != nil && !validText(*input.Name, 255) ||
		input.Status != nil && !validMutationStatus(*input.Status) ||
		input.BidAmountLocalMicro != nil && *input.BidAmountLocalMicro <= 0 ||
		input.DailyBudgetAmountLocalMicro != nil && *input.DailyBudgetAmountLocalMicro <= 0 ||
		input.TotalBudgetAmountLocalMicro != nil && *input.TotalBudgetAmountLocalMicro <= 0 ||
		input.Name == nil && input.Status == nil && input.BidAmountLocalMicro == nil && input.DailyBudgetAmountLocalMicro == nil && input.TotalBudgetAmountLocalMicro == nil {
		return nil, invalidArgument(operation, "Line Item ID or update fields are invalid")
	}
	current, err := client.GetLineItem(ctx, id, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.GetCampaign(ctx, current.CampaignID, options...); err != nil {
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
		return nil, invalidArgument(operation, "daily Line Item budget must not exceed total budget")
	}
	query := make(url.Values)
	if input.Name != nil {
		query.Set("name", *input.Name)
	}
	if input.Status != nil {
		query.Set("entity_status", string(*input.Status))
	}
	setOptionalMicro(query, "bid_amount_local_micro", input.BidAmountLocalMicro)
	setOptionalMicro(query, "daily_budget_amount_local_micro", input.DailyBudgetAmountLocalMicro)
	setOptionalMicro(query, "total_budget_amount_local_micro", input.TotalBudgetAmountLocalMicro)
	var response singleResponse[LineItem]
	if _, err := client.write(ctx, http.MethodPut, client.resourcePath("line_items")+"/"+id, query, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateLineItem(operation, &response.Data, id, current.CampaignID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func validCreateLineItem(input CreateLineItemRequest) bool {
	if !validAdsID(input.CampaignID) || input.Name != "" && !validText(input.Name, 255) || !validObjective(input.Objective) ||
		!validProductType(input.ProductType) || !validPlacements(input.Placements) || !validBidStrategy(input.BidStrategy) ||
		input.StartTime.IsZero() || input.EndTime != nil && (input.EndTime.IsZero() || !input.EndTime.After(input.StartTime)) {
		return false
	}
	if input.BidStrategy == BidStrategyAuto && input.BidAmountLocalMicro != nil ||
		(input.BidStrategy == BidStrategyMax || input.BidStrategy == BidStrategyTarget) && (input.BidAmountLocalMicro == nil || *input.BidAmountLocalMicro <= 0) {
		return false
	}
	if input.DailyBudgetAmountLocalMicro != nil && *input.DailyBudgetAmountLocalMicro <= 0 ||
		input.TotalBudgetAmountLocalMicro != nil && *input.TotalBudgetAmountLocalMicro <= 0 {
		return false
	}
	return input.DailyBudgetAmountLocalMicro == nil || input.TotalBudgetAmountLocalMicro == nil ||
		*input.DailyBudgetAmountLocalMicro <= *input.TotalBudgetAmountLocalMicro
}

func (client *Client) validateLineItem(operation string, value *LineItem, expectedID, expectedCampaignID string) error {
	if !validAdsID(value.ID) || expectedID != "" && value.ID != expectedID || !validAdsID(value.CampaignID) ||
		expectedCampaignID != "" && value.CampaignID != expectedCampaignID {
		return platformContractError(operation, "X returned a missing or mismatched Line Item identity")
	}
	if value.AccountID != "" && value.AccountID != client.adsAccountID {
		return platformContractError(operation, "X returned a Line Item owned by another Ads Account")
	}
	if value.AccountID == "" {
		value.AccountID = client.adsAccountID
	}
	return nil
}

func setOptionalMicro(query url.Values, name string, value *int64) {
	if value != nil {
		query.Set(name, formatInt64(*value))
	}
}

func joinPlacements(values []Placement) string {
	items := make([]string, len(values))
	for index, value := range values {
		items[index] = string(value)
	}
	return strings.Join(items, ",")
}
