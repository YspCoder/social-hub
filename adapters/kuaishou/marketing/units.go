package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListUnits(ctx context.Context, input ListUnitsRequest, options ...socialhub.CallOption) (NumberPage[Unit], error) {
	const operation = "unit_list"
	if !validateIDs(input.IDs, 100) || input.CampaignID < 0 || !validateStatuses(input.PutStatuses) ||
		!validateDatePair(input.StartDate, input.EndDate) || input.Name != "" && !validRequiredText(input.Name, 100) ||
		input.TimeFilterType < 0 || input.TimeFilterType > 1 {
		return NumberPage[Unit]{}, invalidArgument(operation, "filters, dates, statuses, or IDs are invalid")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize, 200)
	if err != nil {
		return NumberPage[Unit]{}, err
	}
	body := map[string]any{"advertiser_id": client.advertiserID, "page": page, "page_size": pageSize}
	if len(input.IDs) > 0 {
		body["unit_ids"] = input.IDs
	}
	if input.CampaignID > 0 {
		body["campaign_id"] = input.CampaignID
	}
	if input.Name != "" {
		body["unit_name"] = input.Name
	}
	if len(input.PutStatuses) > 0 {
		body["put_status_list"] = input.PutStatuses
	}
	if input.StartDate != "" {
		body["start_date"], body["end_date"] = input.StartDate, input.EndDate
		body["time_filter_type"] = input.TimeFilterType
	}
	var response apiEnvelope[struct {
		Details    []Unit `json:"details"`
		TotalCount int64  `json:"total_count"`
	}]
	header, err := client.postJSON(ctx, operation, "/gw/dsp/unit/list", body, &response, options...)
	if err != nil {
		return NumberPage[Unit]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[Unit]{}, err
	}
	for index := range data.Details {
		if err := requireResourceID(operation, 0, data.Details[index].ID); err != nil {
			return NumberPage[Unit]{}, err
		}
		if err := requireAdvertiser(operation, client.advertiserID, data.Details[index].AdvertiserID); err != nil {
			return NumberPage[Unit]{}, err
		}
		data.Details[index].AdvertiserID = client.advertiserID
	}
	return numberPage(data.Details, page, pageSize, data.TotalCount)
}

func (client *Client) CreateUnit(ctx context.Context, input CreateUnitRequest, options ...socialhub.CallOption) (*Unit, error) {
	const operation = "unit_create"
	if !validID(input.CampaignID) || !validRequiredText(input.Name, 100) || !validDate(input.BeginTime) ||
		input.EndTime != "" && (!validDate(input.EndTime) || input.EndTime < input.BeginTime) ||
		!validSceneIDs(input.SceneIDs) || input.UnitType <= 0 || len(input.Target) == 0 {
		return nil, invalidArgument(operation, "campaign, name, dates, scenes, unit type, and target are required")
	}
	if input.BidType != BidTypeCPC && input.BidType != BidTypeOCPM && input.BidType != BidTypeMCB ||
		input.BidType == BidTypeCPC && (input.Bid <= 0 || input.CPABid != 0) ||
		input.BidType == BidTypeOCPM && (input.CPABid <= 0 || input.Bid != 0) || input.Bid < 0 || input.CPABid < 0 {
		return nil, invalidArgument(operation, "bid_type requires its matching positive bid field")
	}
	if input.DayBudget < 0 || input.DayBudget > 0 && len(input.DayBudgetSchedule) > 0 ||
		len(input.DayBudgetSchedule) > 0 && len(input.DayBudgetSchedule) != 7 {
		return nil, invalidArgument(operation, "day budget and a seven-day budget schedule are mutually exclusive")
	}
	fixed := map[string]any{
		"advertiser_id": client.advertiserID, "campaign_id": input.CampaignID, "unit_name": input.Name,
		"begin_time": input.BeginTime, "bid_type": input.BidType, "scene_id": input.SceneIDs,
		"unit_type": input.UnitType, "target": input.Target, "put_status": PutStatusPaused,
	}
	if input.EndTime != "" {
		fixed["end_time"] = input.EndTime
	}
	if input.Bid > 0 {
		fixed["bid"] = input.Bid
	}
	if input.CPABid > 0 {
		fixed["cpa_bid"] = input.CPABid
	}
	if input.DayBudget > 0 {
		fixed["day_budget"] = input.DayBudget
	}
	if len(input.DayBudgetSchedule) > 0 {
		fixed["day_budget_schedule"] = input.DayBudgetSchedule
	}
	body, err := mergeFields(operation, fixed, input.Fields, "unit_id", "campaign_id", "unit_name", "put_status", "target")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		UnitID int64 `json:"unit_id"`
	}]
	header, err := client.postJSON(ctx, operation, "/gw/dsp/unit/create", body, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, 0, data.UnitID); err != nil {
		return nil, err
	}
	target, _ := jsonRaw(input.Target)
	return &Unit{
		ID: data.UnitID, AdvertiserID: client.advertiserID, CampaignID: input.CampaignID, Name: input.Name,
		PutStatus: PutStatusPaused, BidType: input.BidType, Bid: input.Bid, CPABid: input.CPABid,
		DayBudget: input.DayBudget, DayBudgetSchedule: append([]int64(nil), input.DayBudgetSchedule...),
		BeginTime: input.BeginTime, EndTime: input.EndTime, SceneIDs: append([]string(nil), input.SceneIDs...),
		UnitType: input.UnitType, Target: target,
	}, nil
}

func (client *Client) SetUnitStatus(ctx context.Context, unitID int64, status PutStatus, options ...socialhub.CallOption) (BatchResult, error) {
	return client.setStatus(ctx, "unit_status_update", "/v1/ad_unit/update/status", "unit_id", unitID, status, options...)
}
