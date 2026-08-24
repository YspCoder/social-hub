package googleads

import (
	"context"

	"social-hub/pkg/socialhub"
)

const campaignQuery = "SELECT campaign.resource_name, campaign.id, campaign.name, campaign.status, campaign.advertising_channel_type, campaign.campaign_budget, campaign.contains_eu_political_advertising, campaign.serving_status, campaign.primary_status FROM campaign ORDER BY campaign.id"

type campaignRow struct {
	Campaign Campaign `json:"campaign"`
}

type campaignMutateResponse struct {
	Results []struct {
		ResourceName string   `json:"resourceName"`
		Campaign     Campaign `json:"campaign"`
	} `json:"results"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (TokenPage[Campaign], error) {
	const operation = "campaign_list"
	if !validPageToken(input.PageToken) {
		return TokenPage[Campaign]{}, invalidArgument(operation, "page token is invalid")
	}
	response, err := searchRows[campaignRow](ctx, client, operation, campaignQuery, input.PageToken, false, options...)
	if err != nil {
		return TokenPage[Campaign]{}, err
	}
	items := make([]Campaign, len(response.Results))
	for index, row := range response.Results {
		if err := client.validateCampaign(operation, row.Campaign); err != nil {
			return TokenPage[Campaign]{}, err
		}
		items[index] = row.Campaign
	}
	return TokenPage[Campaign]{Items: items, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validRequiredText(input.Name, 255) || !validResourceName(client.customerID, "campaignBudgets", input.BudgetResourceName) ||
		!validEUDeclaration(input.ContainsEUPoliticalAdvertising) {
		return nil, invalidArgument(operation, "name, same-Customer Campaign Budget, and explicit EU political advertising declaration are required")
	}
	fixed := map[string]any{
		"name": input.Name, "status": StatusPaused, "advertisingChannelType": "SEARCH",
		"campaignBudget":                 input.BudgetResourceName,
		"containsEuPoliticalAdvertising": input.ContainsEUPoliticalAdvertising,
		"manualCpc":                      map[string]any{"enhancedCpcEnabled": false},
	}
	if input.NetworkSettings != nil {
		fixed["networkSettings"] = input.NetworkSettings
	}
	resource, err := mergeFields(operation, fixed, input.Fields, "id", "servingStatus", "primaryStatus")
	if err != nil {
		return nil, err
	}
	return client.mutateCampaign(ctx, operation, map[string]any{"create": resource}, options...)
}

func (client *Client) UpdateCampaign(ctx context.Context, resourceName string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validResourceName(client.customerID, "campaigns", resourceName) {
		return nil, invalidArgument(operation, "Campaign resource name is invalid or belongs to another Customer")
	}
	resource := map[string]any{"resourceName": resourceName}
	mask := make([]string, 0, 5)
	if input.Name != nil {
		if !validRequiredText(*input.Name, 255) {
			return nil, invalidArgument(operation, "name is invalid")
		}
		resource["name"] = *input.Name
		mask = append(mask, "name")
	}
	if input.BudgetResourceName != nil {
		if !validResourceName(client.customerID, "campaignBudgets", *input.BudgetResourceName) {
			return nil, invalidArgument(operation, "Campaign Budget is invalid or belongs to another Customer")
		}
		resource["campaignBudget"] = *input.BudgetResourceName
		mask = append(mask, "campaign_budget")
	}
	if input.NetworkSettings != nil {
		resource["networkSettings"] = input.NetworkSettings
		mask = append(mask, "network_settings.target_google_search", "network_settings.target_search_network", "network_settings.target_content_network")
	}
	if len(mask) == 0 {
		return nil, invalidArgument(operation, "at least one mutable field is required")
	}
	return client.mutateCampaign(ctx, operation, map[string]any{
		"update": resource, "updateMask": updateMask(mask),
	}, options...)
}

func (client *Client) SetCampaignStatus(ctx context.Context, resourceName string, status Status, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_status"
	if !validResourceName(client.customerID, "campaigns", resourceName) || !validStatus(status) {
		return nil, invalidArgument(operation, "Campaign resource name or status is invalid")
	}
	return client.mutateCampaign(ctx, operation, map[string]any{
		"update": map[string]any{"resourceName": resourceName, "status": status}, "updateMask": "status",
	}, options...)
}

func (client *Client) RemoveCampaign(ctx context.Context, resourceName string, options ...socialhub.CallOption) error {
	const operation = "campaign_remove"
	if !validResourceName(client.customerID, "campaigns", resourceName) {
		return invalidArgument(operation, "Campaign resource name is invalid or belongs to another Customer")
	}
	result, err := client.mutateCampaign(ctx, operation, map[string]any{"remove": resourceName}, options...)
	if err != nil {
		return err
	}
	if result.ResourceName != resourceName {
		return platformContractError(operation, "Google Ads returned a different removed Campaign")
	}
	return nil
}

func (client *Client) mutateCampaign(ctx context.Context, operation string, mutateOperation map[string]any, options ...socialhub.CallOption) (*Campaign, error) {
	body := map[string]any{"operations": []any{mutateOperation}, "responseContentType": "MUTABLE_RESOURCE"}
	var response campaignMutateResponse
	if _, err := client.postJSON(ctx, operation, client.mutatePath("campaigns"), body, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Results) != 1 {
		return nil, platformContractError(operation, "Google Ads returned an invalid Campaign mutate result count")
	}
	result := response.Results[0]
	if err := requireResourceName(operation, client.customerID, "campaigns", result.ResourceName); err != nil {
		return nil, err
	}
	if result.Campaign.ResourceName == "" {
		result.Campaign.ResourceName = result.ResourceName
	}
	if result.Campaign.ResourceName != result.ResourceName {
		return nil, platformContractError(operation, "Google Ads returned mismatched Campaign resource names")
	}
	if result.Campaign.CampaignBudget != "" && !validResourceName(client.customerID, "campaignBudgets", result.Campaign.CampaignBudget) {
		return nil, platformContractError(operation, "Google Ads returned a Campaign Budget for another Customer")
	}
	return &result.Campaign, nil
}

func (client *Client) validateCampaign(operation string, campaign Campaign) error {
	if err := requireResourceName(operation, client.customerID, "campaigns", campaign.ResourceName); err != nil {
		return err
	}
	if campaign.CampaignBudget != "" && !validResourceName(client.customerID, "campaignBudgets", campaign.CampaignBudget) {
		return platformContractError(operation, "Google Ads returned a Campaign Budget for another Customer")
	}
	return nil
}
