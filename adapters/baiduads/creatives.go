package baiduads

import (
	"context"

	"social-hub/pkg/socialhub"
)

var defaultCreativeFields = []string{
	"creativeId", "adgroupId", "title", "description1", "description2", "pause", "status",
	"mobileDestinationUrl", "mobileDisplayUrl", "pcDestinationUrl", "pcDisplayUrl", "tabs", "createTime",
}

func (client *Client) GetCreative(ctx context.Context, creativeID int64, fields []string, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_get"
	if creativeID <= 0 {
		return nil, invalidArgument(operation, "creative ID must be positive")
	}
	values, err := client.GetCreatives(ctx, GetCreativesRequest{
		IDs: []int64{creativeID}, Fields: fields, IDType: CreativeByID,
	}, options...)
	if err != nil {
		return nil, err
	}
	result, err := oneResult(operation, values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, creativeID, result.ID); err != nil {
		return nil, err
	}
	return result, nil
}

func (client *Client) GetCreatives(ctx context.Context, input GetCreativesRequest, options ...socialhub.CallOption) ([]Creative, error) {
	const operation = "creative_list"
	maximum := 0
	switch input.IDType {
	case CreativeByAdGroupID:
		maximum = 1000
	case CreativeByID:
		maximum = 3000
	default:
		return nil, invalidArgument(operation, "ID type must select AdGroup or Creative IDs")
	}
	if err := validateIDs(operation, input.IDs, maximum, false); err != nil {
		return nil, err
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = defaultCreativeFields
	}
	fields = appendRequiredFields(fields, "creativeId", "adgroupId")
	if err := validateFields(operation, fields, 64); err != nil {
		return nil, err
	}
	var response apiEnvelope[[]Creative]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CreativeService/getCreative", map[string]any{
		"ids": input.IDs, "creativeFields": fields, "idType": input.IDType, "getTemp": boolAsInt(input.GetTemp),
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	for index := range *values {
		if err := requireID(operation, 0, (*values)[index].ID); err != nil {
			return nil, err
		}
		if (*values)[index].AdGroupID <= 0 {
			return nil, platformContractError(operation, "Baidu Ads returned an invalid parent ad group ID")
		}
	}
	return append([]Creative(nil), (*values)...), nil
}

func (client *Client) CreateCreative(ctx context.Context, input CreateCreativeRequest, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_create"
	if input.CampaignID <= 0 || input.AdGroupID <= 0 || !validCreativeCore(
		input.Title, input.Description1, input.Description2,
		input.MobileDestinationURL, input.MobileDisplayURL, input.PCDestinationURL, input.PCDisplayURL, input.Tabs,
	) {
		return nil, invalidArgument(operation, "campaign, ad group, creative text, destinations, or tabs are invalid")
	}
	parent, err := client.GetAdGroup(ctx, input.AdGroupID, []string{"adgroupId", "campaignId", "pause"}, options...)
	if err != nil {
		return nil, err
	}
	if parent.CampaignID != input.CampaignID {
		return nil, invalidArgument(operation, "creative campaign does not own the selected ad group")
	}
	if !parent.Pause {
		return nil, invalidArgument(operation, "parent ad group must be paused before creating a creative")
	}
	fixed := creativeFields(
		input.Title, input.Description1, input.Description2,
		input.MobileDestinationURL, input.MobileDisplayURL, input.PCDestinationURL, input.PCDisplayURL, input.Tabs,
	)
	fixed["campaignId"], fixed["adgroupId"] = input.CampaignID, input.AdGroupID
	resource, err := mergeFields(operation, fixed, input.Fields,
		"campaignId", "adgroupId", "creativeId", "pause", "status", "title", "description1", "description2",
		"mobileDestinationUrl", "mobileDisplayUrl", "pcDestinationUrl", "pcDisplayUrl", "tabs")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[[]Creative]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CreativeService/addCreative", map[string]any{
		"creativeTypes": []any{resource},
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	created, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, 0, created.ID); err != nil {
		return nil, err
	}
	if created.AdGroupID != 0 && created.AdGroupID != input.AdGroupID || created.CampaignID != 0 && created.CampaignID != input.CampaignID {
		return nil, platformContractError(operation, "Baidu Ads returned mismatched creative ownership")
	}
	created.AdGroupID, created.CampaignID = input.AdGroupID, input.CampaignID
	paused, err := client.UpdateCreative(ctx, created.ID, UpdateCreativeRequest{
		Title: input.Title, Description1: input.Description1, Description2: input.Description2, Pause: true,
		MobileDestinationURL: input.MobileDestinationURL, MobileDisplayURL: input.MobileDisplayURL,
		PCDestinationURL: input.PCDestinationURL, PCDisplayURL: input.PCDisplayURL,
		Tabs: append([]int(nil), input.Tabs...), Fields: input.Fields,
	}, options...)
	if err != nil {
		return created, err
	}
	paused.AdGroupID, paused.CampaignID = input.AdGroupID, input.CampaignID
	return paused, nil
}

func (client *Client) UpdateCreative(ctx context.Context, creativeID int64, input UpdateCreativeRequest, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_update"
	if creativeID <= 0 || !validCreativeCore(
		input.Title, input.Description1, input.Description2,
		input.MobileDestinationURL, input.MobileDisplayURL, input.PCDestinationURL, input.PCDisplayURL, input.Tabs,
	) {
		return nil, invalidArgument(operation, "creative ID, text, destinations, or tabs are invalid")
	}
	fixed := creativeFields(
		input.Title, input.Description1, input.Description2,
		input.MobileDestinationURL, input.MobileDisplayURL, input.PCDestinationURL, input.PCDisplayURL, input.Tabs,
	)
	fixed["creativeId"], fixed["pause"] = creativeID, input.Pause
	resource, err := mergeFields(operation, fixed, input.Fields,
		"campaignId", "adgroupId", "creativeId", "pause", "status", "title", "description1", "description2",
		"mobileDestinationUrl", "mobileDisplayUrl", "pcDestinationUrl", "pcDisplayUrl", "tabs")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[[]Creative]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CreativeService/updateCreative", map[string]any{
		"creativeTypes": []any{resource},
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	creative, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, creativeID, creative.ID); err != nil {
		return nil, err
	}
	if creative.Pause != input.Pause {
		return nil, platformContractError(operation, "Baidu Ads returned an unexpected creative pause state")
	}
	return creative, nil
}

func (client *Client) DeleteCreative(ctx context.Context, creativeID int64, options ...socialhub.CallOption) error {
	const operation = "creative_delete"
	if creativeID <= 0 {
		return invalidArgument(operation, "creative ID must be positive")
	}
	var response apiEnvelope[[]Creative]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CreativeService/deleteCreative", map[string]any{
		"creativeIds": []int64{creativeID},
	}, &response, options...)
	if err != nil {
		return err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	creative, err := oneResult(operation, *values)
	if err != nil {
		return err
	}
	return requireID(operation, creativeID, creative.ID)
}

func validCreativeCore(title, description1, description2, mobileDestination, mobileDisplay, pcDestination, pcDisplay string, tabs []int) bool {
	return validText(title, 9, 50) && validText(description1, 9, 80) && validText(description2, 0, 80) &&
		validDestinationURL(mobileDestination, 1024) && validText(mobileDisplay, 1, 36) &&
		validDestinationURL(pcDestination, 1024) && validText(pcDisplay, 1, 36) && validTabs(tabs)
}

func creativeFields(title, description1, description2, mobileDestination, mobileDisplay, pcDestination, pcDisplay string, tabs []int) map[string]any {
	return map[string]any{
		"title": title, "description1": description1, "description2": description2,
		"mobileDestinationUrl": mobileDestination, "mobileDisplayUrl": mobileDisplay,
		"pcDestinationUrl": pcDestination, "pcDisplayUrl": pcDisplay, "tabs": append([]int(nil), tabs...),
	}
}

var _ CreativeWorkflow = (*Client)(nil)
