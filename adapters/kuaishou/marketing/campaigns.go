package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (NumberPage[Campaign], error) {
	const operation = "campaign_list"
	if !validateIDs(input.IDs, 200) || !validateStatuses(input.PutStatuses) || !validateDatePair(input.StartDate, input.EndDate) ||
		input.Name != "" && !validRequiredText(input.Name, 100) || input.TimeFilterType < 0 || input.TimeFilterType > 1 {
		return NumberPage[Campaign]{}, invalidArgument(operation, "filters, dates, statuses, or IDs are invalid")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize, 200)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	body := map[string]any{"advertiser_id": client.advertiserID, "page": page, "page_size": pageSize}
	if len(input.IDs) > 0 {
		body["campaign_ids"] = input.IDs
	}
	if input.Name != "" {
		body["campaign_name"] = input.Name
	}
	if len(input.PutStatuses) > 0 {
		body["put_status_list"] = input.PutStatuses
	}
	if input.Status != nil {
		body["status"] = *input.Status
	}
	if input.StartDate != "" {
		body["start_date"], body["end_date"] = input.StartDate, input.EndDate
		body["time_filter_type"] = input.TimeFilterType
	}
	var response apiEnvelope[struct {
		Details    []Campaign `json:"details"`
		TotalCount int64      `json:"total_count"`
	}]
	header, err := client.postJSON(ctx, operation, "/gw/dsp/campaign/list", body, &response, options...)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	for index := range data.Details {
		if err := requireResourceID(operation, 0, data.Details[index].ID); err != nil {
			return NumberPage[Campaign]{}, err
		}
		if err := requireAdvertiser(operation, client.advertiserID, data.Details[index].AdvertiserID); err != nil {
			return NumberPage[Campaign]{}, err
		}
		data.Details[index].AdvertiserID = client.advertiserID
	}
	return numberPage(data.Details, page, pageSize, data.TotalCount)
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validRequiredText(input.Name, 100) || input.MarketingGoal <= 0 || input.AdType < 0 || input.AdType > 1 || input.BidType < 0 || input.BidType > 1 {
		return nil, invalidArgument(operation, "name, marketing goal, ad type, or bid type is invalid")
	}
	if input.DayBudget < 0 || len(input.DayBudgetSchedule) > 0 && input.DayBudget > 0 ||
		len(input.DayBudgetSchedule) > 0 && len(input.DayBudgetSchedule) != 7 {
		return nil, invalidArgument(operation, "day_budget and a seven-day budget schedule are mutually exclusive")
	}
	for _, value := range input.DayBudgetSchedule {
		if value < 0 {
			return nil, invalidArgument(operation, "budget schedule values must be non-negative")
		}
	}
	fixed := map[string]any{
		"advertiser_id": client.advertiserID, "campaign_name": input.Name, "type": input.MarketingGoal,
		"ad_type": input.AdType, "bid_type": input.BidType, "put_status": PutStatusPaused,
	}
	if input.DayBudget > 0 {
		fixed["day_budget"] = input.DayBudget
	}
	if len(input.DayBudgetSchedule) > 0 {
		fixed["day_budget_schedule"] = input.DayBudgetSchedule
	}
	body, err := mergeFields(operation, fixed, input.Fields, "campaign_id", "put_status", "campaign_name", "type")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		CampaignID int64 `json:"campaign_id"`
	}]
	header, err := client.postJSON(ctx, operation, "/gw/dsp/campaign/create", body, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, 0, data.CampaignID); err != nil {
		return nil, err
	}
	return &Campaign{
		ID: data.CampaignID, AdvertiserID: client.advertiserID, Name: input.Name,
		PutStatus: PutStatusPaused, DayBudget: input.DayBudget,
		DayBudgetSchedule: append([]int64(nil), input.DayBudgetSchedule...), MarketingGoal: input.MarketingGoal,
		AdType: input.AdType, BidType: input.BidType,
	}, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID int64, input UpdateCampaignRequest, options ...socialhub.CallOption) error {
	const operation = "campaign_update"
	if !validID(campaignID) || input.Name == nil && input.DayBudget == nil && input.DayBudgetSchedule == nil && len(input.Fields) == 0 {
		return invalidArgument(operation, "a campaign ID and at least one patch field are required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 100) || input.DayBudget != nil && *input.DayBudget < 0 ||
		input.DayBudget != nil && *input.DayBudget > 0 && input.DayBudgetSchedule != nil && len(*input.DayBudgetSchedule) > 0 ||
		input.DayBudgetSchedule != nil && len(*input.DayBudgetSchedule) > 0 && len(*input.DayBudgetSchedule) != 7 {
		return invalidArgument(operation, "name or budget patch is invalid")
	}
	if input.DayBudgetSchedule != nil {
		for _, value := range *input.DayBudgetSchedule {
			if value < 0 {
				return invalidArgument(operation, "budget schedule values must be non-negative")
			}
		}
	}
	fixed := map[string]any{"advertiser_id": client.advertiserID, "campaign_id": campaignID}
	if input.Name != nil {
		fixed["campaign_name"] = *input.Name
	}
	if input.DayBudget != nil {
		fixed["day_budget"] = *input.DayBudget
	}
	if input.DayBudgetSchedule != nil {
		schedule := append([]int64{}, (*input.DayBudgetSchedule)...)
		fixed["day_budget_schedule"] = schedule
	}
	body, err := mergeFields(operation, fixed, input.Fields, "campaign_id", "put_status")
	if err != nil {
		return err
	}
	var response apiEnvelope[struct {
		CampaignID int64 `json:"campaign_id"`
	}]
	header, err := client.postJSON(ctx, operation, "/gw/dsp/campaign/update", body, &response, options...)
	if err != nil {
		return err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	return requireResourceID(operation, campaignID, data.CampaignID)
}

func (client *Client) SetCampaignStatus(ctx context.Context, campaignID int64, status PutStatus, options ...socialhub.CallOption) (BatchResult, error) {
	return client.setStatus(ctx, "campaign_status_update", "/v1/campaign/update/status", "campaign_id", campaignID, status, options...)
}
