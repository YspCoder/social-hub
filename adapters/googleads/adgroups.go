package googleads

import (
	"context"
	"strconv"

	"social-hub/pkg/socialhub"
)

const adGroupQuery = "SELECT ad_group.resource_name, ad_group.id, ad_group.campaign, ad_group.name, ad_group.status, ad_group.type, ad_group.cpc_bid_micros, ad_group.primary_status FROM ad_group"

type adGroupRow struct {
	AdGroup AdGroup `json:"adGroup"`
}

type adGroupMutateResponse struct {
	Results []struct {
		ResourceName string  `json:"resourceName"`
		AdGroup      AdGroup `json:"adGroup"`
	} `json:"results"`
}

func (client *Client) ListAdGroups(ctx context.Context, input ListAdGroupsRequest, options ...socialhub.CallOption) (TokenPage[AdGroup], error) {
	const operation = "ad_group_list"
	if !validPageToken(input.PageToken) || input.CampaignResourceName != "" && !validResourceName(client.customerID, "campaigns", input.CampaignResourceName) {
		return TokenPage[AdGroup]{}, invalidArgument(operation, "page token or Campaign resource name is invalid")
	}
	query := adGroupQuery
	if input.CampaignResourceName != "" {
		query += " WHERE ad_group.campaign = '" + input.CampaignResourceName + "'"
	}
	query += " ORDER BY ad_group.id"
	response, err := searchRows[adGroupRow](ctx, client, operation, query, input.PageToken, false, options...)
	if err != nil {
		return TokenPage[AdGroup]{}, err
	}
	items := make([]AdGroup, len(response.Results))
	for index, row := range response.Results {
		if err := client.validateAdGroup(operation, row.AdGroup, input.CampaignResourceName); err != nil {
			return TokenPage[AdGroup]{}, err
		}
		items[index] = row.AdGroup
	}
	return TokenPage[AdGroup]{Items: items, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_create"
	if !validResourceName(client.customerID, "campaigns", input.CampaignResourceName) ||
		!validRequiredText(input.Name, 255) || input.CPCBidMicros <= 0 {
		return nil, invalidArgument(operation, "same-Customer Campaign, name, and positive cpc_bid_micros are required")
	}
	fixed := map[string]any{
		"campaign": input.CampaignResourceName, "name": input.Name, "status": StatusPaused,
		"type": "SEARCH_STANDARD", "cpcBidMicros": strconv.FormatInt(input.CPCBidMicros, 10),
	}
	resource, err := mergeFields(operation, fixed, input.Fields, "id", "primaryStatus")
	if err != nil {
		return nil, err
	}
	return client.mutateAdGroup(ctx, operation, map[string]any{"create": resource}, options...)
}

func (client *Client) UpdateAdGroup(ctx context.Context, resourceName string, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_update"
	if !validResourceName(client.customerID, "adGroups", resourceName) {
		return nil, invalidArgument(operation, "Ad Group resource name is invalid or belongs to another Customer")
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
	if input.CPCBidMicros != nil {
		if *input.CPCBidMicros <= 0 {
			return nil, invalidArgument(operation, "cpc_bid_micros must be positive")
		}
		resource["cpcBidMicros"] = strconv.FormatInt(*input.CPCBidMicros, 10)
		mask = append(mask, "cpc_bid_micros")
	}
	if len(mask) == 0 {
		return nil, invalidArgument(operation, "at least one mutable field is required")
	}
	return client.mutateAdGroup(ctx, operation, map[string]any{
		"update": resource, "updateMask": updateMask(mask),
	}, options...)
}

func (client *Client) SetAdGroupStatus(ctx context.Context, resourceName string, status Status, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_status"
	if !validResourceName(client.customerID, "adGroups", resourceName) || !validStatus(status) {
		return nil, invalidArgument(operation, "Ad Group resource name or status is invalid")
	}
	return client.mutateAdGroup(ctx, operation, map[string]any{
		"update": map[string]any{"resourceName": resourceName, "status": status}, "updateMask": "status",
	}, options...)
}

func (client *Client) RemoveAdGroup(ctx context.Context, resourceName string, options ...socialhub.CallOption) error {
	const operation = "ad_group_remove"
	if !validResourceName(client.customerID, "adGroups", resourceName) {
		return invalidArgument(operation, "Ad Group resource name is invalid or belongs to another Customer")
	}
	result, err := client.mutateAdGroup(ctx, operation, map[string]any{"remove": resourceName}, options...)
	if err != nil {
		return err
	}
	if result.ResourceName != resourceName {
		return platformContractError(operation, "Google Ads returned a different removed Ad Group")
	}
	return nil
}

func (client *Client) mutateAdGroup(ctx context.Context, operation string, mutateOperation map[string]any, options ...socialhub.CallOption) (*AdGroup, error) {
	body := map[string]any{"operations": []any{mutateOperation}, "responseContentType": "MUTABLE_RESOURCE"}
	var response adGroupMutateResponse
	if _, err := client.postJSON(ctx, operation, client.mutatePath("adGroups"), body, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Results) != 1 {
		return nil, platformContractError(operation, "Google Ads returned an invalid Ad Group mutate result count")
	}
	result := response.Results[0]
	if err := requireResourceName(operation, client.customerID, "adGroups", result.ResourceName); err != nil {
		return nil, err
	}
	if result.AdGroup.ResourceName == "" {
		result.AdGroup.ResourceName = result.ResourceName
	}
	if result.AdGroup.ResourceName != result.ResourceName {
		return nil, platformContractError(operation, "Google Ads returned mismatched Ad Group resource names")
	}
	if result.AdGroup.Campaign != "" && !validResourceName(client.customerID, "campaigns", result.AdGroup.Campaign) {
		return nil, platformContractError(operation, "Google Ads returned a Campaign for another Customer")
	}
	return &result.AdGroup, nil
}

func (client *Client) validateAdGroup(operation string, adGroup AdGroup, expectedCampaign string) error {
	if err := requireResourceName(operation, client.customerID, "adGroups", adGroup.ResourceName); err != nil {
		return err
	}
	if !validResourceName(client.customerID, "campaigns", adGroup.Campaign) ||
		expectedCampaign != "" && adGroup.Campaign != expectedCampaign {
		return platformContractError(operation, "Google Ads returned an Ad Group for another Campaign or Customer")
	}
	return nil
}
