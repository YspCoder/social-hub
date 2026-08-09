package baiduads

import (
	"context"

	"social-hub/pkg/socialhub"
)

var defaultCampaignFields = []string{
	"campaignId", "campaignName", "budget", "pause", "status", "adType", "marketingTargetId",
	"businessPointId", "equipmentType", "createTime",
}

func (client *Client) GetCampaign(ctx context.Context, campaignID int64, fields []string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if campaignID <= 0 {
		return nil, invalidArgument(operation, "campaign ID must be positive")
	}
	values, err := client.GetCampaigns(ctx, GetCampaignsRequest{IDs: []int64{campaignID}, Fields: fields}, options...)
	if err != nil {
		return nil, err
	}
	result, err := oneResult(operation, values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, campaignID, result.ID); err != nil {
		return nil, err
	}
	return result, nil
}

func (client *Client) GetCampaigns(ctx context.Context, input GetCampaignsRequest, options ...socialhub.CallOption) ([]Campaign, error) {
	const operation = "campaign_list"
	if err := validateIDs(operation, input.IDs, 100, true); err != nil {
		return nil, err
	}
	fields := input.Fields
	if len(fields) == 0 {
		fields = defaultCampaignFields
	}
	fields = appendRequiredFields(fields, "campaignId")
	if err := validateFields(operation, fields, 64); err != nil {
		return nil, err
	}
	body := map[string]any{"campaignFields": fields, "campaignIds": input.IDs}
	if input.AdType != nil {
		if *input.AdType != 0 && *input.AdType != 14 {
			return nil, invalidArgument(operation, "ad type must be 0 or 14")
		}
		body["adType"] = *input.AdType
	}
	var response apiEnvelope[[]Campaign]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CampaignService/getCampaign", body, &response, options...)
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
	}
	return append([]Campaign(nil), (*values)...), nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validText(input.Name, 1, 30) || input.Budget < 0 || input.Budget > 10_000_000 ||
		input.Budget > 0 && input.Budget < 50 || (input.AdType != 0 && input.AdType != 14) ||
		!validMarketingTarget(input.MarketingTargetID) {
		return nil, invalidArgument(operation, "name, budget, ad type, or marketing target is invalid")
	}
	fixed := map[string]any{
		"campaignName": input.Name, "pause": true, "adType": input.AdType,
		"marketingTargetId": input.MarketingTargetID,
	}
	if input.Budget > 0 {
		fixed["budget"] = input.Budget
	}
	resource, err := mergeFields(operation, fixed, input.Fields,
		"campaignId", "campaignName", "budget", "pause", "status", "adType", "marketingTargetId")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[[]Campaign]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CampaignService/addCampaign", map[string]any{
		"campaignTypes": []any{resource}, "adType": input.AdType,
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	campaign, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, 0, campaign.ID); err != nil {
		return nil, err
	}
	if !campaign.Pause {
		return nil, platformContractError(operation, "Baidu Ads did not confirm the campaign as paused")
	}
	return campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID int64, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if campaignID <= 0 || input.Name == nil && input.Budget == nil && input.Pause == nil && len(input.Fields) == 0 {
		return nil, invalidArgument(operation, "campaign ID and at least one update field are required")
	}
	fixed := map[string]any{"campaignId": campaignID}
	if input.Name != nil {
		if !validText(*input.Name, 1, 30) {
			return nil, invalidArgument(operation, "campaign name is invalid")
		}
		fixed["campaignName"] = *input.Name
	}
	if input.Budget != nil {
		if *input.Budget != 0 && (*input.Budget < 50 || *input.Budget > 10_000_000) {
			return nil, invalidArgument(operation, "campaign budget is invalid")
		}
		fixed["budget"] = *input.Budget
	}
	if input.Pause != nil {
		fixed["pause"] = *input.Pause
	}
	resource, err := mergeFields(operation, fixed, input.Fields, "campaignId", "campaignName", "budget", "pause", "status")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[[]Campaign]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CampaignService/updateCampaign", map[string]any{
		"campaignTypes": []any{resource},
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	campaign, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if err := requireID(operation, campaignID, campaign.ID); err != nil {
		return nil, err
	}
	if input.Pause != nil && campaign.Pause != *input.Pause {
		return nil, platformContractError(operation, "Baidu Ads returned an unexpected campaign pause state")
	}
	return campaign, nil
}

func (client *Client) DeleteCampaign(ctx context.Context, campaignID int64, options ...socialhub.CallOption) error {
	const operation = "campaign_delete"
	if campaignID <= 0 {
		return invalidArgument(operation, "campaign ID must be positive")
	}
	var response apiEnvelope[[]Campaign]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/CampaignService/deleteCampaign", map[string]any{
		"campaignIds": []int64{campaignID},
	}, &response, options...)
	if err != nil {
		return err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return err
	}
	campaign, err := oneResult(operation, *values)
	if err != nil {
		return err
	}
	return requireID(operation, campaignID, campaign.ID)
}

func validMarketingTarget(value int) bool {
	switch value {
	case 0, 1, 2, 4, 5:
		return true
	default:
		return false
	}
}

var _ CampaignWorkflow = (*Client)(nil)
