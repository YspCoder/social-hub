package googleads

import (
	"context"
	"strconv"

	"social-hub/pkg/socialhub"
)

const campaignBudgetQuery = "SELECT campaign_budget.resource_name, campaign_budget.id, campaign_budget.name, campaign_budget.amount_micros, campaign_budget.delivery_method, campaign_budget.period, campaign_budget.explicitly_shared, campaign_budget.reference_count, campaign_budget.status FROM campaign_budget ORDER BY campaign_budget.id"

type campaignBudgetRow struct {
	CampaignBudget CampaignBudget `json:"campaignBudget"`
}

type campaignBudgetMutateResponse struct {
	Results []struct {
		ResourceName   string         `json:"resourceName"`
		CampaignBudget CampaignBudget `json:"campaignBudget"`
	} `json:"results"`
}

func (client *Client) ListCampaignBudgets(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (TokenPage[CampaignBudget], error) {
	const operation = "campaign_budget_list"
	if !validPageToken(input.PageToken) {
		return TokenPage[CampaignBudget]{}, invalidArgument(operation, "page token is invalid")
	}
	response, err := searchRows[campaignBudgetRow](ctx, client, operation, campaignBudgetQuery, input.PageToken, false, options...)
	if err != nil {
		return TokenPage[CampaignBudget]{}, err
	}
	items := make([]CampaignBudget, len(response.Results))
	for index, row := range response.Results {
		if err := requireResourceName(operation, client.customerID, "campaignBudgets", row.CampaignBudget.ResourceName); err != nil {
			return TokenPage[CampaignBudget]{}, err
		}
		items[index] = row.CampaignBudget
	}
	return TokenPage[CampaignBudget]{Items: items, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateCampaignBudget(ctx context.Context, input CreateCampaignBudgetRequest, options ...socialhub.CallOption) (*CampaignBudget, error) {
	const operation = "campaign_budget_create"
	if !validRequiredText(input.Name, 255) || input.AmountMicros <= 0 {
		return nil, invalidArgument(operation, "name and positive amount_micros are required")
	}
	fixed := map[string]any{
		"name": input.Name, "amountMicros": strconv.FormatInt(input.AmountMicros, 10), "deliveryMethod": "STANDARD",
	}
	if input.ExplicitlyShared != nil {
		fixed["explicitlyShared"] = *input.ExplicitlyShared
	}
	resource, err := mergeFields(operation, fixed, input.Fields, "period", "type", "totalAmountMicros")
	if err != nil {
		return nil, err
	}
	return client.mutateCampaignBudget(ctx, operation, map[string]any{"create": resource}, options...)
}

func (client *Client) UpdateCampaignBudget(ctx context.Context, resourceName string, input UpdateCampaignBudgetRequest, options ...socialhub.CallOption) (*CampaignBudget, error) {
	const operation = "campaign_budget_update"
	if !validResourceName(client.customerID, "campaignBudgets", resourceName) {
		return nil, invalidArgument(operation, "campaign budget resource name is invalid or belongs to another Customer")
	}
	resource := map[string]any{"resourceName": resourceName}
	mask := make([]string, 0, 2)
	if input.Name != nil {
		if !validRequiredText(*input.Name, 255) {
			return nil, invalidArgument(operation, "name is invalid")
		}
		resource["name"] = *input.Name
		mask = append(mask, "name")
	}
	if input.AmountMicros != nil {
		if *input.AmountMicros <= 0 {
			return nil, invalidArgument(operation, "amount_micros must be positive")
		}
		resource["amountMicros"] = strconv.FormatInt(*input.AmountMicros, 10)
		mask = append(mask, "amount_micros")
	}
	if len(mask) == 0 {
		return nil, invalidArgument(operation, "at least one mutable field is required")
	}
	return client.mutateCampaignBudget(ctx, operation, map[string]any{
		"update": resource, "updateMask": updateMask(mask),
	}, options...)
}

func (client *Client) RemoveCampaignBudget(ctx context.Context, resourceName string, options ...socialhub.CallOption) error {
	const operation = "campaign_budget_remove"
	if !validResourceName(client.customerID, "campaignBudgets", resourceName) {
		return invalidArgument(operation, "campaign budget resource name is invalid or belongs to another Customer")
	}
	result, err := client.mutateCampaignBudget(ctx, operation, map[string]any{"remove": resourceName}, options...)
	if err != nil {
		return err
	}
	if result.ResourceName != resourceName {
		return platformContractError(operation, "Google Ads returned a different removed Campaign Budget")
	}
	return nil
}

func (client *Client) mutateCampaignBudget(ctx context.Context, operation string, mutateOperation map[string]any, options ...socialhub.CallOption) (*CampaignBudget, error) {
	body := map[string]any{
		"operations": []any{mutateOperation}, "responseContentType": "MUTABLE_RESOURCE",
	}
	var response campaignBudgetMutateResponse
	if _, err := client.postJSON(ctx, operation, client.mutatePath("campaignBudgets"), body, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Results) != 1 {
		return nil, platformContractError(operation, "Google Ads returned an invalid Campaign Budget mutate result count")
	}
	result := response.Results[0]
	if err := requireResourceName(operation, client.customerID, "campaignBudgets", result.ResourceName); err != nil {
		return nil, err
	}
	if result.CampaignBudget.ResourceName == "" {
		result.CampaignBudget.ResourceName = result.ResourceName
	}
	if result.CampaignBudget.ResourceName != result.ResourceName {
		return nil, platformContractError(operation, "Google Ads returned mismatched Campaign Budget resource names")
	}
	return &result.CampaignBudget, nil
}
