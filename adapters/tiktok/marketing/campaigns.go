package marketing

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (NumberPage[Campaign], error) {
	const operation = "campaign_list"
	if !validateIDs(input.IDs, 100) || !validateFields(input.Fields, 100) ||
		input.Name != "" && !validRequiredText(input.Name, 512) ||
		input.ObjectiveType != "" && !validEnumToken(input.ObjectiveType) ||
		input.PrimaryStatus != "" && !validEnumToken(input.PrimaryStatus) ||
		input.SecondaryStatus != "" && !validEnumToken(input.SecondaryStatus) {
		return NumberPage[Campaign]{}, invalidArgument(operation, "campaign filters or fields are invalid")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	query := url.Values{
		"advertiser_id": {client.advertiserID}, "page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)},
	}
	filtering := map[string]any{}
	if len(input.IDs) > 0 {
		filtering["campaign_ids"] = input.IDs
	}
	if input.Name != "" {
		filtering["campaign_name"] = input.Name
	}
	if input.ObjectiveType != "" {
		filtering["objective_type"] = input.ObjectiveType
	}
	if input.PrimaryStatus != "" {
		filtering["primary_status"] = input.PrimaryStatus
	}
	if input.SecondaryStatus != "" {
		filtering["secondary_status"] = input.SecondaryStatus
	}
	if len(filtering) > 0 {
		if err := setJSONQuery(query, "filtering", filtering, operation); err != nil {
			return NumberPage[Campaign]{}, err
		}
	}
	if len(input.Fields) > 0 {
		if err := setJSONQuery(query, "fields", input.Fields, operation); err != nil {
			return NumberPage[Campaign]{}, err
		}
	}
	var response apiEnvelope[struct {
		List     []Campaign `json:"list"`
		PageInfo *pageInfo  `json:"page_info"`
	}]
	header, err := client.getJSON(ctx, operation, "/v1.3/campaign/get/", query, &response, options...)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[Campaign]{}, err
	}
	for index := range data.List {
		if err := requireResourceID(operation, "", data.List[index].ID); err != nil {
			return NumberPage[Campaign]{}, err
		}
		if err := requireAdvertiser(operation, client.advertiserID, data.List[index].AdvertiserID); err != nil {
			return NumberPage[Campaign]{}, err
		}
		data.List[index].AdvertiserID = client.advertiserID
	}
	return numberPage(operation, data.List, data.PageInfo)
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validRequiredText(input.Name, 512) || !validEnumToken(input.ObjectiveType) ||
		input.CampaignType != "" && !validEnumToken(input.CampaignType) ||
		input.BudgetMode != "" && !validEnumToken(input.BudgetMode) || input.Budget < 0 ||
		input.RequestID != "" && !validOpaque(input.RequestID, 128) {
		return nil, invalidArgument(operation, "campaign name, objective, budget, type, mode, or request ID is invalid")
	}
	if input.ObjectiveType == "RF_REACH" {
		return nil, unsupported(operation, "Reach & Frequency campaigns cannot satisfy the adapter's paused-creation guarantee; use the dedicated RF workflow outside this adapter version")
	}
	if len(input.SpecialIndustries) > 16 {
		return nil, invalidArgument(operation, "special_industries exceeds the supported request size")
	}
	for _, value := range input.SpecialIndustries {
		if !validEnumToken(value) {
			return nil, invalidArgument(operation, "special industries must be uppercase enum tokens")
		}
	}
	fixed := map[string]any{
		"advertiser_id": client.advertiserID, "campaign_name": input.Name,
		"objective_type": input.ObjectiveType, "operation_status": StatusDisable,
	}
	if input.CampaignType != "" {
		fixed["campaign_type"] = input.CampaignType
	}
	if input.BudgetMode != "" {
		fixed["budget_mode"] = input.BudgetMode
	}
	if input.Budget > 0 {
		fixed["budget"] = input.Budget
	}
	if input.BudgetOptimizeOn != nil {
		fixed["budget_optimize_on"] = *input.BudgetOptimizeOn
	}
	if len(input.SpecialIndustries) > 0 {
		fixed["special_industries"] = input.SpecialIndustries
	}
	if input.RequestID != "" {
		fixed["request_id"] = input.RequestID
	}
	body, err := mergeFields(operation, fixed, input.Fields,
		"campaign_id", "campaign_name", "objective_type", "operation_status")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[Campaign]
	header, err := client.postJSON(ctx, operation, "/v1.3/campaign/create/", body, &response, options...)
	if err != nil {
		return nil, err
	}
	campaign, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, "", campaign.ID); err != nil {
		return nil, err
	}
	if err := requireAdvertiser(operation, client.advertiserID, campaign.AdvertiserID); err != nil {
		return nil, err
	}
	if campaign.OperationStatus != "" && campaign.OperationStatus != StatusDisable {
		return nil, platformContractError(operation, "TikTok did not create the campaign paused")
	}
	campaign.AdvertiserID, campaign.OperationStatus = client.advertiserID, StatusDisable
	return campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validID(campaignID) || input.Name == nil && input.Budget == nil && len(input.Fields) == 0 {
		return nil, invalidArgument(operation, "a campaign ID and at least one patch field are required")
	}
	if input.Name != nil && !validRequiredText(*input.Name, 512) || input.Budget != nil && *input.Budget < 0 {
		return nil, invalidArgument(operation, "one or more campaign patch fields are invalid")
	}
	fixed := map[string]any{"advertiser_id": client.advertiserID, "campaign_id": campaignID}
	if input.Name != nil {
		fixed["campaign_name"] = *input.Name
	}
	if input.Budget != nil {
		fixed["budget"] = *input.Budget
	}
	body, err := mergeFields(operation, fixed, input.Fields, "campaign_id", "campaign_name", "budget", "operation_status")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[Campaign]
	header, err := client.postJSON(ctx, operation, "/v1.3/campaign/update/", body, &response, options...)
	if err != nil {
		return nil, err
	}
	campaign, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, campaignID, campaign.ID); err != nil {
		return nil, err
	}
	if err := requireAdvertiser(operation, client.advertiserID, campaign.AdvertiserID); err != nil {
		return nil, err
	}
	campaign.AdvertiserID = client.advertiserID
	return campaign, nil
}

func (client *Client) SetCampaignStatus(ctx context.Context, campaignID string, status OperationStatus, options ...socialhub.CallOption) (BatchResult, error) {
	return client.setStatus(ctx, "campaign_status_update", "/v1.3/campaign/status/update/", "campaign_ids", campaignID, status, options...)
}
