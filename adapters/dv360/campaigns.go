package dv360

import (
	"context"
	"net/url"
	"sort"
	"strings"

	"social-hub/pkg/socialhub"
)

var campaignOrderFields = map[string]struct{}{
	"displayName": {}, "entityStatus": {}, "updateTime": {},
}

type campaignCreatePayload struct {
	DisplayName     string           `json:"displayName"`
	EntityStatus    EntityStatus     `json:"entityStatus"`
	CampaignGoal    CampaignGoal     `json:"campaignGoal"`
	CampaignFlight  CampaignFlight   `json:"campaignFlight"`
	FrequencyCap    FrequencyCap     `json:"frequencyCap"`
	CampaignBudgets []CampaignBudget `json:"campaignBudgets,omitempty"`
}

type campaignPatchPayload struct {
	DisplayName     *string           `json:"displayName,omitempty"`
	EntityStatus    *EntityStatus     `json:"entityStatus,omitempty"`
	CampaignGoal    *CampaignGoal     `json:"campaignGoal,omitempty"`
	CampaignFlight  *CampaignFlight   `json:"campaignFlight,omitempty"`
	FrequencyCap    *FrequencyCap     `json:"frequencyCap,omitempty"`
	CampaignBudgets *[]CampaignBudget `json:"campaignBudgets,omitempty"`
}

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_get"
	if !validID(campaignID) {
		return Campaign{}, invalidArgument(operation, "campaign ID must be a positive string-encoded integer")
	}
	var campaign Campaign
	path := client.campaignsPath() + "/" + campaignID
	if err := client.getJSON(ctx, operation, path, nil, &campaign, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign(operation, campaign); err != nil {
		return Campaign{}, err
	}
	if campaign.CampaignID != campaignID {
		return Campaign{}, platformContractError(operation, "DV360 returned a different campaign")
	}
	return campaign, nil
}

func (client *Client) ListCampaigns(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Campaign], error) {
	const operation = "campaign_list"
	if !validPage(input, 200, campaignOrderFields) {
		return Page[Campaign]{}, invalidArgument(operation, "pagination, filter, or order is invalid")
	}
	var response listCampaignsResponse
	if err := client.getJSON(ctx, operation, client.campaignsPath(), listQuery(input), &response, options...); err != nil {
		return Page[Campaign]{}, err
	}
	seen := make(map[string]struct{}, len(response.Campaigns))
	for _, campaign := range response.Campaigns {
		if err := client.validateCampaign(operation, campaign); err != nil {
			return Page[Campaign]{}, err
		}
		if _, exists := seen[campaign.CampaignID]; exists {
			return Page[Campaign]{}, platformContractError(operation, "DV360 returned duplicate campaigns")
		}
		seen[campaign.CampaignID] = struct{}{}
	}
	if !validPageToken(response.NextPageToken) {
		return Page[Campaign]{}, platformContractError(operation, "DV360 returned an invalid page token")
	}
	return Page[Campaign]{Items: response.Campaigns, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_create"
	if !validDisplayName(input.DisplayName) || !validCampaignGoal(input.CampaignGoal) ||
		!validCampaignFlight(input.CampaignFlight) || !validFrequencyCap(input.FrequencyCap) ||
		!validCampaignBudgets(input.CampaignBudgets) {
		return Campaign{}, invalidArgument(operation, "campaign fields are invalid")
	}
	payload := campaignCreatePayload{
		DisplayName: input.DisplayName, EntityStatus: EntityStatusPaused,
		CampaignGoal: input.CampaignGoal, CampaignFlight: input.CampaignFlight,
		FrequencyCap: input.FrequencyCap, CampaignBudgets: append([]CampaignBudget(nil), input.CampaignBudgets...),
	}
	var campaign Campaign
	if err := client.postJSON(ctx, operation, client.campaignsPath(), payload, &campaign, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign(operation, campaign); err != nil {
		return Campaign{}, err
	}
	if campaign.DisplayName != input.DisplayName || campaign.EntityStatus != EntityStatusPaused {
		return Campaign{}, platformContractError(operation, "new DV360 campaign was not returned paused with the requested name")
	}
	return campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_update"
	mask, err := validateCampaignPatch(campaignID, input)
	if err != nil {
		return Campaign{}, withOperation(err, operation)
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return Campaign{}, withOperation(err, operation)
	}
	if input.EntityStatus != nil && *input.EntityStatus == EntityStatusActive {
		advertiser, err := client.GetAdvertiser(ctx, options...)
		if err != nil {
			return Campaign{}, withOperation(err, operation)
		}
		if advertiser.EntityStatus != EntityStatusActive {
			return Campaign{}, conflictError(operation, "campaign cannot be activated while its advertiser is not active")
		}
	}
	payload := campaignPatchPayload{
		DisplayName: input.DisplayName, EntityStatus: input.EntityStatus, CampaignGoal: input.CampaignGoal,
		CampaignFlight: input.CampaignFlight, FrequencyCap: input.FrequencyCap, CampaignBudgets: input.CampaignBudgets,
	}
	query := url.Values{"updateMask": {mask}}
	var campaign Campaign
	if err := client.patchJSON(ctx, operation, client.campaignsPath()+"/"+campaignID, query, payload, &campaign, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign(operation, campaign); err != nil {
		return Campaign{}, err
	}
	if campaign.CampaignID != campaignID || !campaignMatchesPatch(campaign, input) {
		return Campaign{}, platformContractError(operation, "DV360 returned a campaign that does not match the update")
	}
	return campaign, nil
}

func validateCampaignPatch(campaignID string, input UpdateCampaignRequest) (string, error) {
	if !validID(campaignID) {
		return "", invalidArgument("campaign_update", "campaign ID is invalid")
	}
	fields := make([]string, 0, 6)
	if input.DisplayName != nil {
		if !validDisplayName(*input.DisplayName) {
			return "", invalidArgument("campaign_update", "display name is invalid")
		}
		fields = append(fields, "displayName")
	}
	if input.EntityStatus != nil {
		if !validUpdateEntityStatus(*input.EntityStatus) {
			return "", invalidArgument("campaign_update", "entity status is invalid")
		}
		fields = append(fields, "entityStatus")
	}
	if input.CampaignGoal != nil {
		if !validCampaignGoal(*input.CampaignGoal) {
			return "", invalidArgument("campaign_update", "campaign goal is invalid")
		}
		fields = append(fields, "campaignGoal")
	}
	if input.CampaignFlight != nil {
		if !validCampaignFlight(*input.CampaignFlight) {
			return "", invalidArgument("campaign_update", "campaign flight is invalid")
		}
		fields = append(fields, "campaignFlight")
	}
	if input.FrequencyCap != nil {
		if !validFrequencyCap(*input.FrequencyCap) {
			return "", invalidArgument("campaign_update", "frequency cap is invalid")
		}
		fields = append(fields, "frequencyCap")
	}
	if input.CampaignBudgets != nil {
		if !validCampaignBudgets(*input.CampaignBudgets) {
			return "", invalidArgument("campaign_update", "campaign budgets are invalid")
		}
		fields = append(fields, "campaignBudgets")
	}
	if len(fields) == 0 {
		return "", invalidArgument("campaign_update", "at least one field must be updated")
	}
	return joinMask(fields), nil
}

func (client *Client) validateCampaign(operation string, campaign Campaign) error {
	if !validID(campaign.CampaignID) || !validID(campaign.AdvertiserID) || !validDisplayName(campaign.DisplayName) ||
		!validReadEntityStatus(campaign.EntityStatus) {
		return platformContractError(operation, "DV360 returned an invalid campaign")
	}
	if campaign.AdvertiserID != client.advertiserID {
		return ownershipError(operation, "campaign")
	}
	return nil
}

func campaignMatchesPatch(campaign Campaign, input UpdateCampaignRequest) bool {
	return (input.DisplayName == nil || campaign.DisplayName == *input.DisplayName) &&
		(input.EntityStatus == nil || campaign.EntityStatus == *input.EntityStatus)
}

func (client *Client) campaignsPath() string {
	return "/v4/advertisers/" + client.advertiserID + "/campaigns"
}

func joinMask(fields []string) string {
	sort.Strings(fields)
	return strings.Join(fields, ",")
}
