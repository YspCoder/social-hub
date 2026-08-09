package outbrain

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (CampaignPage, error) {
	if !validListCampaigns(input) {
		return CampaignPage{}, invalidArgument("list_campaigns", "invalid Campaign list filters")
	}
	query := url.Values{"fetch": {"all"}}
	if input.IncludeArchived {
		query.Set("includeArchived", "true")
	}
	if input.FromBudgetStartDate != "" {
		query.Set("fromBudgetStartDate", input.FromBudgetStartDate)
		query.Set("toBudgetEndDate", input.ToBudgetEndDate)
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	if input.DaysToLookBack > 0 {
		query.Set("daysToLookBackForChanges", strconv.Itoa(input.DaysToLookBack))
	}
	var envelope struct {
		Campaigns []Campaign `json:"campaigns"`
		Count     int        `json:"count"`
	}
	path := "marketers/" + url.PathEscape(client.marketerID) + "/campaigns"
	if err := client.getJSON(ctx, "list_campaigns", path, query, &envelope, options...); err != nil {
		return CampaignPage{}, err
	}
	if envelope.Count != len(envelope.Campaigns) {
		return CampaignPage{}, platformContractError("list_campaigns", "campaign count does not match results")
	}
	for _, campaign := range envelope.Campaigns {
		if err := client.validateCampaign("list_campaigns", campaign, ""); err != nil {
			return CampaignPage{}, err
		}
	}
	return CampaignPage{Items: envelope.Campaigns, Count: envelope.Count}, nil
}

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (Campaign, error) {
	if !validPathID(campaignID) {
		return Campaign{}, invalidArgument("get_campaign", "campaign ID is invalid")
	}
	var campaign Campaign
	if err := client.getJSON(ctx, "get_campaign", "campaigns/"+url.PathEscape(campaignID), nil, &campaign, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign("get_campaign", campaign, campaignID); err != nil {
		return Campaign{}, err
	}
	return campaign, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	if !validCreateCampaign(input) {
		return Campaign{}, invalidArgument("create_campaign", "Campaign fields are invalid")
	}
	if _, err := client.ensureBudgetOwned(ctx, input.BudgetID, options...); err != nil {
		return Campaign{}, err
	}
	payload := struct {
		Name               string             `json:"name"`
		CPC                float64            `json:"cpc"`
		Enabled            bool               `json:"enabled"`
		BudgetID           string             `json:"budgetId"`
		Targeting          *CampaignTargeting `json:"targeting,omitempty"`
		SuffixTrackingCode string             `json:"suffixTrackingCode,omitempty"`
		Objective          string             `json:"objective,omitempty"`
		CreativeFormat     string             `json:"creativeFormat,omitempty"`
	}{
		Name: input.Name, CPC: input.CPC, Enabled: false, BudgetID: input.BudgetID,
		SuffixTrackingCode: input.SuffixTrackingCode,
		Objective:          input.Objective, CreativeFormat: input.CreativeFormat,
	}
	if !emptyTargeting(input.Targeting) {
		payload.Targeting = &input.Targeting
	}
	var campaign Campaign
	if err := client.postJSON(ctx, "create_campaign", "campaigns", payload, &campaign, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign("create_campaign", campaign, ""); err != nil {
		return Campaign{}, err
	}
	if campaign.Enabled || campaign.LiveStatus.CampaignOnAir || campaign.Name != input.Name || campaign.Budget.ID != input.BudgetID {
		return Campaign{}, platformContractError("create_campaign", "Campaign was not created in the requested disabled state")
	}
	return campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	if !validPathID(campaignID) || !validUpdateCampaign(input) {
		return Campaign{}, invalidArgument("update_campaign", "campaign ID or update fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return Campaign{}, err
	}
	var campaign Campaign
	if err := client.putJSON(ctx, "update_campaign", "campaigns/"+url.PathEscape(campaignID), input, &campaign, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign("update_campaign", campaign, campaignID); err != nil {
		return Campaign{}, err
	}
	return campaign, nil
}

func (client *Client) SetCampaignEnabled(ctx context.Context, campaignID string, enabled bool, options ...socialhub.CallOption) (Campaign, error) {
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return Campaign{}, err
	}
	if campaign.Enabled == enabled {
		return campaign, nil
	}
	if enabled {
		links, err := client.listPromotedLinksUnchecked(ctx, campaignID, ListPromotedLinksRequest{Limit: 500}, options...)
		if err != nil {
			return Campaign{}, err
		}
		if len(links.Items) == 0 {
			return Campaign{}, invalidArgument("set_campaign_enabled", "Campaign requires at least one PromotedLink")
		}
		if links.TotalCount != len(links.Items) {
			return Campaign{}, invalidArgument("set_campaign_enabled", "Campaign has more than 500 PromotedLinks and cannot be completely safety-checked")
		}
		for _, link := range links.Items {
			if link.Enabled || link.Archived || !link.Approved() {
				return Campaign{}, invalidArgument("set_campaign_enabled", "all PromotedLinks must be disabled, unarchived, and approved")
			}
		}
	}
	var updated Campaign
	payload := struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled}
	if err := client.putJSON(ctx, "set_campaign_enabled", "campaigns/"+url.PathEscape(campaignID), payload, &updated, options...); err != nil {
		return Campaign{}, err
	}
	if err := client.validateCampaign("set_campaign_enabled", updated, campaignID); err != nil {
		return Campaign{}, err
	}
	if updated.Enabled != enabled {
		return Campaign{}, platformContractError("set_campaign_enabled", "Campaign enabled state does not match request")
	}
	return updated, nil
}

func (client *Client) validateCampaign(operation string, campaign Campaign, requestedID string) error {
	if !validPathID(campaign.ID) || (requestedID != "" && campaign.ID != requestedID) || campaign.MarketerID != client.marketerID ||
		!validText(campaign.Name, 1024) || campaign.CPC <= 0 || !validPathID(campaign.Budget.ID) {
		return platformContractError(operation, "Campaign response ownership or fields are invalid")
	}
	return nil
}

func validListCampaigns(input ListCampaignsRequest) bool {
	if !validPage(input.Limit, input.Offset, 50) || input.DaysToLookBack < 0 || input.DaysToLookBack > 3650 {
		return false
	}
	if (input.FromBudgetStartDate == "") != (input.ToBudgetEndDate == "") {
		return false
	}
	return input.FromBudgetStartDate == "" || validDateWindow(input.FromBudgetStartDate, input.ToBudgetEndDate)
}

func validCreateCampaign(input CreateCampaignRequest) bool {
	return validText(input.Name, 1024) && input.CPC > 0 && input.CPC <= 1_000_000 && validPathID(input.BudgetID) &&
		validTargeting(input.Targeting) && (input.SuffixTrackingCode == "" || validText(input.SuffixTrackingCode, 4096)) &&
		(input.Objective == "" || validText(input.Objective, 128)) && (input.CreativeFormat == "" || validText(input.CreativeFormat, 128))
}

func emptyTargeting(value CampaignTargeting) bool {
	return len(value.Platform) == 0 && len(value.Locations) == 0 && len(value.OperatingSystems) == 0 && len(value.Browsers) == 0
}

func validUpdateCampaign(input UpdateCampaignRequest) bool {
	if input.Name == nil && input.CPC == nil && input.SuffixTrackingCode == nil && input.Objective == nil {
		return false
	}
	return validOptionalText(input.Name, 1024) && validPositive(input.CPC) &&
		validOptionalText(input.SuffixTrackingCode, 4096) && validOptionalText(input.Objective, 128)
}
