package baiduads

import (
	"context"

	"social-hub/pkg/socialhub"
)

var defaultAdGroupFields = []string{
	"adgroupId", "campaignId", "adgroupName", "maxPrice", "pause", "status", "adType", "productSetId", "paPrice", "createTime",
}

func (client *Client) GetAdGroup(ctx context.Context, adGroupID int64, fields []string, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_get"
	if adGroupID <= 0 {
		return nil, invalidArgument(operation, "ad group ID must be positive")
	}
	values, err := client.GetAdGroups(ctx, GetAdGroupsRequest{
		IDs: []int64{adGroupID}, Fields: fields, IDType: AdGroupByID,
	}, options...)
	if err != nil {
		return nil, err
	}
	result, err := oneResult(operation, values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, adGroupID, result.ID); err != nil {
		return nil, err
	}
	return result, nil
}

func (client *Client) GetAdGroups(ctx context.Context, input GetAdGroupsRequest, options ...socialhub.CallOption) ([]AdGroup, error) {
	const operation = "adgroup_list"
	maximum := 0
	switch input.IDType {
	case AdGroupByCampaignID:
		maximum = 100
	case AdGroupByID:
		maximum = 5000
	default:
		return nil, invalidArgument(operation, "ID type must select Campaign or AdGroup IDs")
	}
	if err := validateIDs(operation, input.IDs, maximum, false); err != nil {
		return nil, err
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = defaultAdGroupFields
	}
	fields = appendRequiredFields(fields, "adgroupId", "campaignId")
	if err := validateFields(operation, fields, 64); err != nil {
		return nil, err
	}
	var response apiEnvelope[[]AdGroup]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/AdgroupService/getAdgroup", map[string]any{
		"ids": input.IDs, "adgroupFields": fields, "idType": input.IDType, "getTemp": boolAsInt(input.GetTemp),
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	for index := range *values {
		if err := requireID(operation, 0, (*values)[index].ID); err != nil || (*values)[index].CampaignID <= 0 {
			if err != nil {
				return nil, err
			}
			return nil, platformContractError(operation, "Baidu Ads returned an invalid parent campaign ID")
		}
	}
	return append([]AdGroup(nil), (*values)...), nil
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_create"
	if input.CampaignID <= 0 || !validText(input.Name, 1, 30) || input.MaxPrice <= 0 || input.MaxPrice > 999.99 ||
		(input.AdType != 0 && input.AdType != 14) {
		return nil, invalidArgument(operation, "campaign, name, bid, or ad type is invalid")
	}
	fixed := map[string]any{
		"campaignId": input.CampaignID, "adgroupName": input.Name, "maxPrice": input.MaxPrice,
		"pause": true, "adType": input.AdType,
	}
	resource, err := mergeFields(operation, fixed, input.Fields,
		"adgroupId", "campaignId", "adgroupName", "maxPrice", "pause", "status", "adType")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[[]AdGroup]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/AdgroupService/addAdgroup", map[string]any{
		"adgroupTypes": []any{resource},
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	adGroup, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, 0, adGroup.ID); err != nil || adGroup.CampaignID != input.CampaignID {
		if err != nil {
			return nil, err
		}
		return nil, platformContractError(operation, "Baidu Ads returned a mismatched parent campaign ID")
	}
	if !adGroup.Pause {
		return nil, platformContractError(operation, "Baidu Ads did not confirm the ad group as paused")
	}
	return adGroup, nil
}

func (client *Client) UpdateAdGroup(ctx context.Context, adGroupID int64, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_update"
	if adGroupID <= 0 || input.Name == nil && input.MaxPrice == nil && input.Pause == nil && len(input.Fields) == 0 {
		return nil, invalidArgument(operation, "ad group ID and at least one update field are required")
	}
	fixed := map[string]any{"adgroupId": adGroupID}
	if input.Name != nil {
		if !validText(*input.Name, 1, 30) {
			return nil, invalidArgument(operation, "ad group name is invalid")
		}
		fixed["adgroupName"] = *input.Name
	}
	if input.MaxPrice != nil {
		if *input.MaxPrice <= 0 || *input.MaxPrice > 999.99 {
			return nil, invalidArgument(operation, "ad group max price is invalid")
		}
		fixed["maxPrice"] = *input.MaxPrice
	}
	if input.Pause != nil {
		fixed["pause"] = *input.Pause
	}
	resource, err := mergeFields(operation, fixed, input.Fields, "adgroupId", "campaignId", "adgroupName", "maxPrice", "pause", "status")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[[]AdGroup]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/AdgroupService/updateAdgroup", map[string]any{
		"adgroupTypes": []any{resource},
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	adGroup, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, adGroupID, adGroup.ID); err != nil {
		return nil, err
	}
	if input.Pause != nil && adGroup.Pause != *input.Pause {
		return nil, platformContractError(operation, "Baidu Ads returned an unexpected ad group pause state")
	}
	return adGroup, nil
}

func (client *Client) DeleteAdGroup(ctx context.Context, adGroupID int64, options ...socialhub.CallOption) error {
	const operation = "adgroup_delete"
	if adGroupID <= 0 {
		return invalidArgument(operation, "ad group ID must be positive")
	}
	var response apiEnvelope[[]AdGroup]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/AdgroupService/deleteAdgroup", map[string]any{
		"adgroupIds": []int64{adGroupID},
	}, &response, options...)
	if err != nil {
		return err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	adGroup, err := oneResult(operation, *values)
	if err != nil {
		return err
	}
	return requireID(operation, adGroupID, adGroup.ID)
}

var _ AdGroupWorkflow = (*Client)(nil)
