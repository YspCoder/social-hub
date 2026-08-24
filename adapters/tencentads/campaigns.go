package tencentads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

var campaignFields = []string{
	"campaign_id", "campaign_name", "configured_status", "campaign_type", "promoted_object_type",
	"daily_budget", "total_budget", "created_time", "last_modified_time", "is_deleted",
}

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (NumberPage[Campaign], error) {
	const operation = "campaign_list"
	page, pageSize, err := validateList(input.Fields, input.Filtering, input.Page, input.PageSize)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = campaignFields
	}
	fields = appendRequiredFields(fields, "campaign_id")
	query := url.Values{
		"account_id": {strconv.FormatInt(client.advertiserID, 10)}, "page": {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(pageSize)}, "is_deleted": {strconv.FormatBool(input.IncludeDeleted)},
	}
	if err := setJSONQuery(query, "fields", fields, operation); err != nil {
		return NumberPage[Campaign]{}, err
	}
	if len(input.Filtering) > 0 {
		if err := setJSONQuery(query, "filtering", input.Filtering, operation); err != nil {
			return NumberPage[Campaign]{}, err
		}
	}
	var response apiEnvelope[struct {
		List     []Campaign `json:"list"`
		PageInfo *pageInfo  `json:"page_info"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodGet, "/campaigns/get", query, nil, &response, options...)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	if err := validatePageInfo(operation, data.PageInfo); err != nil {
		return NumberPage[Campaign]{}, err
	}
	for index := range data.List {
		if err := requireResourceID(operation, 0, data.List[index].ID); err != nil {
			return NumberPage[Campaign]{}, err
		}
		if err := requireAccount(operation, client.advertiserID, data.List[index].AccountID); err != nil {
			return NumberPage[Campaign]{}, err
		}
		data.List[index].AccountID = client.advertiserID
	}
	return numberPage(data.List, data.PageInfo), nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validRequiredText(input.Name, 512) || !validEnum(string(input.CampaignType)) || !validEnum(string(input.PromotedObjectType)) {
		return nil, invalidArgument(operation, "name, campaign type, and promoted object type are required")
	}
	if input.DailyBudget < 0 || input.TotalBudget < 0 || input.DailyBudget > 0 && input.TotalBudget > 0 {
		return nil, invalidArgument(operation, "daily and total budgets must be non-negative and mutually exclusive")
	}
	fixed := map[string]any{
		"account_id": client.advertiserID, "campaign_name": input.Name,
		"campaign_type": input.CampaignType, "promoted_object_type": input.PromotedObjectType,
		"configured_status": ConfiguredStatusSuspend,
	}
	if input.DailyBudget > 0 {
		fixed["daily_budget"] = input.DailyBudget
	}
	if input.TotalBudget > 0 {
		fixed["total_budget"] = input.TotalBudget
	}
	body, err := mergeFields(operation, fixed, input.Fields, "campaign_id", "configured_status", "campaign_name")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		CampaignID int64 `json:"campaign_id"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/campaigns/add", nil, body, &response, options...)
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
		ID: data.CampaignID, AccountID: client.advertiserID, Name: input.Name,
		CampaignType: input.CampaignType, PromotedObjectType: input.PromotedObjectType,
		ConfiguredStatus: ConfiguredStatusSuspend, DailyBudget: input.DailyBudget, TotalBudget: input.TotalBudget,
	}, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID int64, input UpdateCampaignRequest, options ...socialhub.CallOption) error {
	const operation = "campaign_update"
	if !validID(campaignID) || input.Name == nil && input.DailyBudget == nil && input.TotalBudget == nil && len(input.Fields) == 0 {
		return invalidArgument(operation, "a campaign ID and at least one patch field are required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) {
		return invalidArgument(operation, "campaign name is invalid")
	}
	if input.DailyBudget != nil && *input.DailyBudget < 0 || input.TotalBudget != nil && *input.TotalBudget < 0 ||
		input.DailyBudget != nil && input.TotalBudget != nil && *input.DailyBudget > 0 && *input.TotalBudget > 0 {
		return invalidArgument(operation, "budgets must be non-negative and mutually exclusive when supplied together")
	}
	fixed := map[string]any{"account_id": client.advertiserID, "campaign_id": campaignID}
	if input.Name != nil {
		fixed["campaign_name"] = *input.Name
	}
	if input.DailyBudget != nil {
		fixed["daily_budget"] = *input.DailyBudget
	}
	if input.TotalBudget != nil {
		fixed["total_budget"] = *input.TotalBudget
	}
	body, err := mergeFields(operation, fixed, input.Fields, "configured_status", "campaign_name", "campaign_id")
	if err != nil {
		return err
	}
	var response apiEnvelope[struct {
		CampaignID int64 `json:"campaign_id"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/campaigns/update", nil, body, &response, options...)
	if err != nil {
		return err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	return requireResourceID(operation, campaignID, data.CampaignID)
}

func (client *Client) SetCampaignStatus(ctx context.Context, campaignID int64, status ConfiguredStatus, options ...socialhub.CallOption) error {
	const operation = "campaign_status_update"
	if !validID(campaignID) || !validStatus(status) {
		return invalidArgument(operation, "a campaign ID and NORMAL or SUSPEND status are required")
	}
	body := map[string]any{
		"account_id":                    client.advertiserID,
		"update_configured_status_spec": []map[string]any{{"campaign_id": campaignID, "configured_status": status}},
	}
	var response apiEnvelope[struct {
		List []struct {
			Code       int64  `json:"code"`
			Message    string `json:"message"`
			MessageCN  string `json:"message_cn"`
			CampaignID int64  `json:"campaign_id"`
		} `json:"list"`
		FailIDs []int64 `json:"fail_id_list"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodPost, "/campaigns/update_configured_status", nil, body, &response, options...)
	if err != nil {
		return err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	for _, failed := range data.FailIDs {
		if failed == campaignID {
			return platformContractError(operation, "Tencent Ads reported the campaign status update as failed")
		}
	}
	for _, item := range data.List {
		if item.CampaignID == campaignID {
			if item.Code != 0 {
				return businessError(operation, item.Code, firstNonEmpty(item.MessageCN, item.Message), header)
			}
			return nil
		}
	}
	return platformContractError(operation, "Tencent Ads did not confirm the campaign status update")
}
