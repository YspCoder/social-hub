package microsoftads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAdGroups(ctx context.Context, campaignID string, options ...socialhub.CallOption) ([]AdGroup, error) {
	const operation = "list_ad_groups"
	if !validNumericID(campaignID) {
		return nil, invalidArgument(operation, "campaign ID must be a nonzero numeric ID")
	}
	var response struct {
		AdGroups []AdGroup `json:"AdGroups"`
	}
	_, err := client.postJSON(ctx, operation, client.campaign, "/AdGroups/QueryByCampaignId", struct {
		CampaignID string `json:"CampaignId"`
	}{CampaignID: campaignID}, &response, options...)
	if err != nil {
		return nil, err
	}
	return response.AdGroups, nil
}

func (client *Client) GetAdGroup(ctx context.Context, campaignID, adGroupID string, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "get_ad_group"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) {
		return nil, invalidArgument(operation, "campaign and ad group IDs must be nonzero numeric IDs")
	}
	var response struct {
		AdGroups      []AdGroup     `json:"AdGroups"`
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/AdGroups/QueryByIds", struct {
		CampaignID string   `json:"CampaignId"`
		AdGroupIDs []string `json:"AdGroupIds"`
	}{CampaignID: campaignID, AdGroupIDs: []string{adGroupID}}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.AdGroups) != 1 || response.AdGroups[0].ID != adGroupID {
		return nil, platformContractError(operation, "response ad group does not match requested campaign and ID")
	}
	return &response.AdGroups[0], nil
}

func (client *Client) CreateAdGroup(ctx context.Context, campaignID string, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "create_ad_group"
	if !validNumericID(campaignID) || !validRequiredText(input.Name, 256) ||
		(input.CPCBid != nil && *input.CPCBid <= 0) ||
		!validOptionalText(input.Language, 16) || !validNetwork(input.Network, true) {
		return nil, invalidArgument(operation, "campaign ID, name, bid, language, or network is invalid")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return nil, err
	}
	if input.Language == "" && len(campaign.Languages) == 0 {
		return nil, invalidArgument(operation, "language is required when the campaign has no language")
	}
	network := input.Network
	if network == "" {
		network = NetworkOwnedAndOperatedAndSyndicatedSearch
	}
	payload := adGroupWrite{Name: &input.Name, Language: optionalStringPointer(input.Language), Network: &network, Status: statusPointer(StatusPaused)}
	if input.CPCBid != nil {
		payload.CPCBid = &Bid{Amount: *input.CPCBid}
	}
	var response struct {
		AdGroupIDs    []*string     `json:"AdGroupIds"`
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.postJSON(ctx, operation, client.campaign, "/AdGroups", struct {
		CampaignID string         `json:"CampaignId"`
		AdGroups   []adGroupWrite `json:"AdGroups"`
	}{CampaignID: campaignID, AdGroups: []adGroupWrite{payload}}, &response, options...)
	if err != nil {
		return nil, err
	}
	if err := checkPartialErrors(operation, header, response.PartialErrors); err != nil {
		return nil, err
	}
	if len(response.AdGroupIDs) != 1 || response.AdGroupIDs[0] == nil || !validNumericID(*response.AdGroupIDs[0]) {
		return nil, platformContractError(operation, "response did not contain one ad group ID")
	}
	return client.GetAdGroup(ctx, campaignID, *response.AdGroupIDs[0], options...)
}

func (client *Client) UpdateAdGroup(ctx context.Context, campaignID, adGroupID string, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "update_ad_group"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || input.empty() ||
		(input.Name != nil && !validRequiredText(*input.Name, 256)) ||
		(input.CPCBid != nil && *input.CPCBid <= 0) ||
		(input.Language != nil && !validRequiredText(*input.Language, 16)) ||
		(input.Network != nil && !validNetwork(*input.Network, false)) {
		return nil, invalidArgument(operation, "IDs and at least one valid update field are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	payload := adGroupWrite{ID: adGroupID, Name: input.Name, Language: input.Language, Network: input.Network}
	if input.CPCBid != nil {
		payload.CPCBid = &Bid{Amount: *input.CPCBid}
	}
	if err := client.updateAdGroup(ctx, operation, campaignID, payload, options...); err != nil {
		return nil, err
	}
	return client.GetAdGroup(ctx, campaignID, adGroupID, options...)
}

func (client *Client) SetAdGroupStatus(ctx context.Context, campaignID, adGroupID string, status Status, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "set_ad_group_status"
	if !validNumericID(campaignID) || !validNumericID(adGroupID) || !validStatus(status) {
		return nil, invalidArgument(operation, "IDs and Active or Paused status are required")
	}
	if err := client.validateAccount(ctx, options...); err != nil {
		return nil, err
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	if err := client.updateAdGroup(ctx, operation, campaignID, adGroupWrite{ID: adGroupID, Status: &status}, options...); err != nil {
		return nil, err
	}
	return client.GetAdGroup(ctx, campaignID, adGroupID, options...)
}

type adGroupWrite struct {
	ID       string   `json:"Id,omitempty"`
	Name     *string  `json:"Name,omitempty"`
	Status   *Status  `json:"Status,omitempty"`
	CPCBid   *Bid     `json:"CpcBid,omitempty"`
	Language *string  `json:"Language,omitempty"`
	Network  *Network `json:"Network,omitempty"`
}

func (client *Client) updateAdGroup(ctx context.Context, operation, campaignID string, payload adGroupWrite, options ...socialhub.CallOption) error {
	var response struct {
		PartialErrors []wireFailure `json:"PartialErrors"`
	}
	header, err := client.putJSON(ctx, operation, "/AdGroups", struct {
		CampaignID string         `json:"CampaignId"`
		AdGroups   []adGroupWrite `json:"AdGroups"`
	}{CampaignID: campaignID, AdGroups: []adGroupWrite{payload}}, &response, options...)
	if err != nil {
		return err
	}
	return checkPartialErrors(operation, header, response.PartialErrors)
}

func (input UpdateAdGroupRequest) empty() bool {
	return input.Name == nil && input.CPCBid == nil && input.Language == nil && input.Network == nil
}

func validNetwork(value Network, allowEmpty bool) bool {
	return (allowEmpty && value == "") || value == NetworkOwnedAndOperatedAndSyndicatedSearch ||
		value == NetworkOwnedAndOperatedOnly || value == NetworkSyndicatedSearchOnly
}

func optionalStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
