package marketing

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAdGroups(ctx context.Context, input ListAdGroupsRequest, options ...socialhub.CallOption) (NumberPage[AdGroup], error) {
	const operation = "adgroup_list"
	if !validateIDs(input.IDs, 100) || !validateIDs(input.CampaignIDs, 100) || !validateFields(input.Fields, 100) ||
		input.Name != "" && !validRequiredText(input.Name, 512) ||
		input.PrimaryStatus != "" && !validEnumToken(input.PrimaryStatus) ||
		input.SecondaryStatus != "" && !validEnumToken(input.SecondaryStatus) {
		return NumberPage[AdGroup]{}, invalidArgument(operation, "Ad Group filters or fields are invalid")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize)
	if err != nil {
		return NumberPage[AdGroup]{}, err
	}
	query := url.Values{
		"advertiser_id": {client.advertiserID}, "page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	filtering := map[string]any{}
	if len(input.IDs) > 0 {
		filtering["adgroup_ids"] = input.IDs
	}
	if len(input.CampaignIDs) > 0 {
		filtering["campaign_ids"] = input.CampaignIDs
	}
	if input.Name != "" {
		filtering["adgroup_name"] = input.Name
	}
	if input.PrimaryStatus != "" {
		filtering["primary_status"] = input.PrimaryStatus
	}
	if input.SecondaryStatus != "" {
		filtering["secondary_status"] = input.SecondaryStatus
	}
	if len(filtering) > 0 {
		if err := setJSONQuery(query, "filtering", filtering, operation); err != nil {
			return NumberPage[AdGroup]{}, err
		}
	}
	if len(input.Fields) > 0 {
		if err := setJSONQuery(query, "fields", input.Fields, operation); err != nil {
			return NumberPage[AdGroup]{}, err
		}
	}
	var response apiEnvelope[struct {
		List     []AdGroup `json:"list"`
		PageInfo *pageInfo `json:"page_info"`
	}]
	header, err := client.getJSON(ctx, operation, "/v1.3/adgroup/get/", query, &response, options...)
	if err != nil {
		return NumberPage[AdGroup]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[AdGroup]{}, err
	}
	for index := range data.List {
		if err := requireResourceID(operation, "", data.List[index].ID); err != nil {
			return NumberPage[AdGroup]{}, err
		}
		if err := requireAdvertiser(operation, client.advertiserID, data.List[index].AdvertiserID); err != nil {
			return NumberPage[AdGroup]{}, err
		}
		data.List[index].AdvertiserID = client.advertiserID
	}
	return numberPage(operation, data.List, data.PageInfo)
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_create"
	if !validID(input.CampaignID) || !validRequiredText(input.Name, 512) ||
		input.PromotionType != "" && !validEnumToken(input.PromotionType) ||
		input.PlacementType != "" && !validEnumToken(input.PlacementType) ||
		input.BudgetMode != "" && !validEnumToken(input.BudgetMode) ||
		input.ScheduleType != "" && !validEnumToken(input.ScheduleType) ||
		input.OptimizationGoal != "" && !validEnumToken(input.OptimizationGoal) ||
		input.BillingEvent != "" && !validEnumToken(input.BillingEvent) ||
		input.BidType != "" && !validEnumToken(input.BidType) || input.Budget < 0 || input.BidPrice < 0 ||
		!validDateTime(input.ScheduleStart) || !validDateTime(input.ScheduleEnd) ||
		(input.ScheduleStart == "") != (input.ScheduleEnd == "") ||
		input.ScheduleStart != "" && input.ScheduleStart > input.ScheduleEnd ||
		input.RequestID != "" && !validOpaque(input.RequestID, 128) {
		return nil, invalidArgument(operation, "Ad Group identifiers, enums, budget, schedule, bid, or request ID are invalid")
	}
	if len(input.Placements) > 20 || len(input.LocationIDs) > 1000 {
		return nil, invalidArgument(operation, "placements or location_ids exceed endpoint limits")
	}
	for _, value := range input.Placements {
		if !validEnumToken(value) {
			return nil, invalidArgument(operation, "placements must be uppercase enum tokens")
		}
	}
	if !validateIDs(input.LocationIDs, 1000) {
		return nil, invalidArgument(operation, "location_ids must be unique numeric strings")
	}
	fixed := map[string]any{
		"advertiser_id": client.advertiserID, "campaign_id": input.CampaignID,
		"adgroup_name": input.Name, "operation_status": StatusDisable,
	}
	optionalStrings := map[string]string{
		"promotion_type": input.PromotionType, "placement_type": input.PlacementType,
		"budget_mode": input.BudgetMode, "schedule_type": input.ScheduleType,
		"schedule_start_time": input.ScheduleStart, "schedule_end_time": input.ScheduleEnd,
		"optimization_goal": input.OptimizationGoal, "billing_event": input.BillingEvent,
		"bid_type": input.BidType, "request_id": input.RequestID,
	}
	for key, value := range optionalStrings {
		if value != "" {
			fixed[key] = value
		}
	}
	if len(input.Placements) > 0 {
		fixed["placements"] = input.Placements
	}
	if len(input.LocationIDs) > 0 {
		fixed["location_ids"] = input.LocationIDs
	}
	if input.Budget > 0 {
		fixed["budget"] = input.Budget
	}
	if input.BidPrice > 0 {
		fixed["bid_price"] = input.BidPrice
	}
	body, err := mergeFields(operation, fixed, input.Fields,
		"adgroup_id", "campaign_id", "adgroup_name", "operation_status")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[AdGroup]
	header, err := client.postJSON(ctx, operation, "/v1.3/adgroup/create/", body, &response, options...)
	if err != nil {
		return nil, err
	}
	adGroup, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, "", adGroup.ID); err != nil {
		return nil, err
	}
	if err := requireAdvertiser(operation, client.advertiserID, adGroup.AdvertiserID); err != nil {
		return nil, err
	}
	if adGroup.CampaignID != "" && adGroup.CampaignID != input.CampaignID {
		return nil, platformContractError(operation, "TikTok returned an Ad Group for another campaign")
	}
	if adGroup.OperationStatus != "" && adGroup.OperationStatus != StatusDisable {
		return nil, platformContractError(operation, "TikTok did not create the Ad Group paused")
	}
	adGroup.AdvertiserID, adGroup.CampaignID, adGroup.OperationStatus = client.advertiserID, input.CampaignID, StatusDisable
	return adGroup, nil
}

func (client *Client) UpdateAdGroup(ctx context.Context, adGroupID string, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_update"
	if !validID(adGroupID) || input.Name == nil && input.Budget == nil && len(input.Fields) == 0 {
		return nil, invalidArgument(operation, "an Ad Group ID and at least one patch field are required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) || input.Budget != nil && *input.Budget < 0 {
		return nil, invalidArgument(operation, "one or more Ad Group patch fields are invalid")
	}
	fixed := map[string]any{"advertiser_id": client.advertiserID, "adgroup_id": adGroupID}
	if input.Name != nil {
		fixed["adgroup_name"] = *input.Name
	}
	if input.Budget != nil {
		fixed["budget"] = *input.Budget
	}
	body, err := mergeFields(operation, fixed, input.Fields, "adgroup_id", "adgroup_name", "budget", "operation_status")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[AdGroup]
	header, err := client.postJSON(ctx, operation, "/v1.3/adgroup/update/", body, &response, options...)
	if err != nil {
		return nil, err
	}
	adGroup, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, adGroupID, adGroup.ID); err != nil {
		return nil, err
	}
	if err := requireAdvertiser(operation, client.advertiserID, adGroup.AdvertiserID); err != nil {
		return nil, err
	}
	adGroup.AdvertiserID = client.advertiserID
	return adGroup, nil
}

func (client *Client) SetAdGroupStatus(ctx context.Context, adGroupID string, status OperationStatus, options ...socialhub.CallOption) (BatchResult, error) {
	return client.setStatus(ctx, "adgroup_status_update", "/v1.3/adgroup/status/update/", "adgroup_ids", adGroupID, status, options...)
}
