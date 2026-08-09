package tencentads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

var adGroupFields = []string{
	"adgroup_id", "campaign_id", "adgroup_name", "configured_status", "system_status", "status",
	"promoted_object_type", "promoted_object_id", "billing_event", "optimization_goal", "bid_amount",
	"daily_budget", "begin_date", "end_date", "created_time", "last_modified_time", "is_deleted",
}

func (client *Client) ListAdGroups(ctx context.Context, input ListAdGroupsRequest, options ...socialhub.CallOption) (NumberPage[AdGroup], error) {
	const operation = "adgroup_list"
	page, pageSize, err := validateList(input.Fields, input.Filtering, input.Page, input.PageSize)
	if err != nil {
		return NumberPage[AdGroup]{}, err
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = adGroupFields
	}
	fields = appendRequiredFields(fields, "adgroup_id", "campaign_id")
	query := url.Values{
		"account_id": {strconv.FormatInt(client.advertiserID, 10)}, "page": {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(pageSize)}, "is_deleted": {strconv.FormatBool(input.IncludeDeleted)},
	}
	if err := setJSONQuery(query, "fields", fields, operation); err != nil {
		return NumberPage[AdGroup]{}, err
	}
	if len(input.Filtering) > 0 {
		if err := setJSONQuery(query, "filtering", input.Filtering, operation); err != nil {
			return NumberPage[AdGroup]{}, err
		}
	}
	var response apiEnvelope[struct {
		List     []AdGroup `json:"list"`
		PageInfo *pageInfo `json:"page_info"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodGet, "/adgroups/get", query, nil, &response, options...)
	if err != nil {
		return NumberPage[AdGroup]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[AdGroup]{}, err
	}
	if err := validatePageInfo(operation, data.PageInfo); err != nil {
		return NumberPage[AdGroup]{}, err
	}
	for index := range data.List {
		if err := requireResourceID(operation, 0, data.List[index].ID); err != nil || !validID(data.List[index].CampaignID) {
			return NumberPage[AdGroup]{}, platformContractError(operation, "Tencent Ads returned an invalid ad group or campaign ID")
		}
		if err := requireAccount(operation, client.advertiserID, data.List[index].AccountID); err != nil {
			return NumberPage[AdGroup]{}, err
		}
		data.List[index].AccountID = client.advertiserID
	}
	return numberPage(data.List, data.PageInfo), nil
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_create"
	if !validID(input.CampaignID) || !validRequiredText(input.Name, 512) || !validEnum(string(input.PromotedObjectType)) ||
		!validEnum(string(input.BillingEvent)) || !validEnum(string(input.OptimizationGoal)) || input.BidAmount < 0 {
		return nil, invalidArgument(operation, "campaign, name, promoted object type, billing event, optimization goal, and bid are invalid")
	}
	if !validDate(input.BeginDate) || !validDate(input.EndDate) || input.BeginDate > input.EndDate {
		return nil, invalidArgument(operation, "begin and end dates must be ordered YYYY-MM-DD values")
	}
	fixed := map[string]any{
		"account_id": client.advertiserID, "campaign_id": input.CampaignID, "adgroup_name": input.Name,
		"promoted_object_type": input.PromotedObjectType, "billing_event": input.BillingEvent,
		"optimization_goal": input.OptimizationGoal, "begin_date": input.BeginDate, "end_date": input.EndDate,
		"configured_status": ConfiguredStatusSuspend,
	}
	if input.BidAmount > 0 {
		fixed["bid_amount"] = input.BidAmount
	}
	body, err := mergeFields(operation, fixed, input.Fields, "adgroup_id", "configured_status", "adgroup_name", "campaign_id")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		AdGroupID int64 `json:"adgroup_id"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/adgroups/add", nil, body, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, 0, data.AdGroupID); err != nil {
		return nil, err
	}
	return &AdGroup{
		ID: data.AdGroupID, AccountID: client.advertiserID, CampaignID: input.CampaignID, Name: input.Name,
		ConfiguredStatus: ConfiguredStatusSuspend, PromotedObjectType: input.PromotedObjectType,
		BillingEvent: input.BillingEvent, OptimizationGoal: input.OptimizationGoal, BidAmount: input.BidAmount,
		BeginDate: input.BeginDate, EndDate: input.EndDate,
	}, nil
}

func (client *Client) UpdateAdGroup(ctx context.Context, adGroupID int64, input UpdateAdGroupRequest, options ...socialhub.CallOption) error {
	const operation = "adgroup_update"
	if !validID(adGroupID) || input.Name == nil && input.BidAmount == nil && input.DailyBudget == nil && input.EndDate == nil && len(input.Fields) == 0 {
		return invalidArgument(operation, "an ad group ID and at least one patch field are required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) || input.BidAmount != nil && *input.BidAmount < 0 ||
		input.DailyBudget != nil && *input.DailyBudget < 0 || input.EndDate != nil && !validDate(*input.EndDate) {
		return invalidArgument(operation, "one or more ad group patch fields are invalid")
	}
	fixed := map[string]any{"account_id": client.advertiserID, "adgroup_id": adGroupID}
	if input.Name != nil {
		fixed["adgroup_name"] = *input.Name
	}
	if input.BidAmount != nil {
		fixed["bid_amount"] = *input.BidAmount
	}
	if input.DailyBudget != nil {
		fixed["daily_budget"] = *input.DailyBudget
	}
	if input.EndDate != nil {
		fixed["end_date"] = *input.EndDate
	}
	body, err := mergeFields(operation, fixed, input.Fields, "configured_status", "adgroup_name", "adgroup_id")
	if err != nil {
		return err
	}
	var response apiEnvelope[struct {
		AdGroupID int64 `json:"adgroup_id"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/adgroups/update", nil, body, &response, options...)
	if err != nil {
		return err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	return requireResourceID(operation, adGroupID, data.AdGroupID)
}

func (client *Client) SetAdGroupStatus(ctx context.Context, adGroupID int64, status ConfiguredStatus, options ...socialhub.CallOption) error {
	const operation = "adgroup_status_update"
	if !validID(adGroupID) || !validStatus(status) {
		return invalidArgument(operation, "an ad group ID and NORMAL or SUSPEND status are required")
	}
	body := map[string]any{
		"account_id":                    client.advertiserID,
		"update_configured_status_spec": []map[string]any{{"adgroup_id": adGroupID, "configured_status": status}},
	}
	var response apiEnvelope[struct {
		List []struct {
			Code      int64  `json:"code"`
			Message   string `json:"message"`
			MessageCN string `json:"message_cn"`
			AdGroupID int64  `json:"adgroup_id"`
		} `json:"list"`
		FailIDs []int64 `json:"fail_id_list"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/adgroups/update_configured_status", nil, body, &response, options...)
	if err != nil {
		return err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	for _, failed := range data.FailIDs {
		if failed == adGroupID {
			return platformContractError(operation, "Tencent Ads reported the ad group status update as failed")
		}
	}
	for _, item := range data.List {
		if item.AdGroupID == adGroupID {
			if item.Code != 0 {
				return businessError(operation, item.Code, firstNonEmpty(item.MessageCN, item.Message), header)
			}
			return nil
		}
	}
	return platformContractError(operation, "Tencent Ads did not confirm the ad group status update")
}
