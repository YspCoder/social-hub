package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

var allCampaignStatuses = []Status{
	StatusActive, StatusPaused, StatusDraft, StatusArchived, StatusCompleted, StatusCanceled, StatusPendingDeletion, StatusRemoved,
}

type campaignPage struct {
	Elements []Campaign `json:"elements"`
	Metadata struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"metadata"`
}

type createCampaignPayload struct {
	Account                  string            `json:"account"`
	CampaignGroup            string            `json:"campaignGroup"`
	AssociatedEntity         string            `json:"associatedEntity"`
	Name                     string            `json:"name"`
	Objective                ObjectiveType     `json:"objectiveType"`
	CostType                 CostType          `json:"costType"`
	CreativeSelection        string            `json:"creativeSelection"`
	DailyBudget              Money             `json:"dailyBudget"`
	TotalBudget              *Money            `json:"totalBudget,omitempty"`
	UnitCost                 Money             `json:"unitCost"`
	Locale                   Locale            `json:"locale"`
	RunSchedule              RunSchedule       `json:"runSchedule"`
	TargetingCriteria        TargetingCriteria `json:"targetingCriteria"`
	AudienceExpansionEnabled bool              `json:"audienceExpansionEnabled"`
	OffsiteDeliveryEnabled   bool              `json:"offsiteDeliveryEnabled"`
	Status                   Status            `json:"status"`
	Type                     string            `json:"type"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[Campaign], error) {
	const operation = "campaigns_list"
	if !validPage(input.Cursor, input.MaxResults, 1000) || !validStatuses(input.Statuses, validStatus) {
		return socialhub.Page[Campaign]{}, invalidArgument(operation, "statuses, page token, or page size is invalid")
	}
	statuses := input.Statuses
	if len(statuses) == 0 {
		statuses = allCampaignStatuses
	}
	var response campaignPage
	if _, err := client.getJSON(ctx, operation, client.resourcePath("adCampaigns"), searchQuery(statuses, input.Cursor, input.MaxResults), "", &response, options...); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	for index := range response.Elements {
		if err := client.validateCampaign(operation, &response.Elements[index], "", ""); err != nil {
			return socialhub.Page[Campaign]{}, err
		}
	}
	return cursorPage(response.Elements, response.Metadata.NextPageToken), nil
}

func (client *Client) GetCampaign(ctx context.Context, id string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validNumericID(id) {
		return nil, invalidArgument(operation, "Campaign ID must be numeric")
	}
	var response Campaign
	if _, err := client.getJSON(ctx, operation, client.resourcePath("adCampaigns")+"/"+id, "", "", &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response, id, ""); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validNumericID(input.CampaignGroupID) || !validAssociatedEntityURN(input.AssociatedEntityURN) || !validText(input.Name, 200) ||
		!validObjective(input.Objective) || !validCostType(input.CostType) || !validMoney(input.DailyBudget) ||
		input.TotalBudget != nil && !validMoney(*input.TotalBudget) || !validMoney(input.UnitCost) || !validLocale(input.Locale) ||
		!validSchedule(input.RunSchedule) || !validTargeting(input.TargetingCriteria) || !campaignCurrenciesMatch(input) {
		return nil, invalidArgument(operation, "Campaign Group, entity, campaign fields, budget, schedule, locale, or targeting is invalid")
	}
	payload := createCampaignPayload{
		Account: client.accountURN(), CampaignGroup: campaignGroupURNPrefix + input.CampaignGroupID,
		AssociatedEntity: input.AssociatedEntityURN, Name: input.Name, Objective: input.Objective,
		CostType: input.CostType, CreativeSelection: "OPTIMIZED", DailyBudget: input.DailyBudget,
		TotalBudget: input.TotalBudget, UnitCost: input.UnitCost, Locale: input.Locale, RunSchedule: input.RunSchedule,
		TargetingCriteria: input.TargetingCriteria, AudienceExpansionEnabled: input.AudienceExpansionEnabled,
		OffsiteDeliveryEnabled: input.OffsiteDeliveryEnabled, Status: StatusDraft, Type: "SPONSORED_UPDATES",
	}
	metadata, err := client.writeJSON(ctx, operation, client.resourcePath("adCampaigns"), "", payload, nil, options...)
	if err != nil {
		return nil, err
	}
	id, err := numericIDFromHeader(operation, metadata, campaignURNPrefix)
	if err != nil {
		return nil, err
	}
	campaign, err := client.GetCampaign(ctx, id, options...)
	if err != nil {
		return nil, err
	}
	if campaign.CampaignGroup != campaignGroupURNPrefix+input.CampaignGroupID {
		return nil, platformContractError(operation, "LinkedIn returned a Campaign in another Campaign Group")
	}
	return campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, id string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validNumericID(id) || input.Name != nil && !validText(*input.Name, 200) ||
		input.DailyBudget != nil && !validMoney(*input.DailyBudget) || input.TotalBudget != nil && !validMoney(*input.TotalBudget) ||
		input.EndTime != nil && *input.EndTime <= 0 || input.Name == nil && input.DailyBudget == nil && input.TotalBudget == nil && input.EndTime == nil {
		return nil, invalidArgument(operation, "Campaign ID or update fields are invalid")
	}
	if input.DailyBudget != nil && input.TotalBudget != nil && input.DailyBudget.CurrencyCode != input.TotalBudget.CurrencyCode {
		return nil, invalidArgument(operation, "Campaign budget currencies must match")
	}
	set := map[string]any{}
	if input.Name != nil {
		set["name"] = *input.Name
	}
	if input.DailyBudget != nil {
		set["dailyBudget"] = input.DailyBudget
	}
	if input.TotalBudget != nil {
		set["totalBudget"] = input.TotalBudget
	}
	if input.EndTime != nil {
		current, err := client.GetCampaign(ctx, id, options...)
		if err != nil {
			return nil, err
		}
		if current.RunSchedule.Start <= 0 || *input.EndTime <= current.RunSchedule.Start {
			return nil, invalidArgument(operation, "Campaign end time must follow its current start time")
		}
		set["runSchedule"] = RunSchedule{Start: current.RunSchedule.Start, End: *input.EndTime}
	}
	return client.updateCampaign(ctx, operation, id, set, options...)
}

func (client *Client) SetCampaignStatus(ctx context.Context, id string, status Status, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_status"
	if !validNumericID(id) || !validMutationStatus(status) {
		return nil, invalidArgument(operation, "Campaign ID or status is invalid")
	}
	return client.updateCampaign(ctx, operation, id, map[string]any{"status": status}, options...)
}

func (client *Client) ArchiveCampaign(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "campaign_archive"
	if !validNumericID(id) {
		return invalidArgument(operation, "Campaign ID must be numeric")
	}
	_, err := client.updateCampaign(ctx, operation, id, map[string]any{"status": StatusArchived}, options...)
	return err
}

func (client *Client) updateCampaign(ctx context.Context, operation, id string, set map[string]any, options ...socialhub.CallOption) (*Campaign, error) {
	payload := map[string]any{"patch": map[string]any{"$set": set}}
	if _, err := client.writeJSON(ctx, operation, client.resourcePath("adCampaigns")+"/"+id, "PARTIAL_UPDATE", payload, nil, options...); err != nil {
		return nil, err
	}
	return client.GetCampaign(ctx, id, options...)
}

func (client *Client) validateCampaign(operation string, value *Campaign, expectedID, expectedGroupID string) error {
	if !validNumericID(string(value.ID)) || expectedID != "" && string(value.ID) != expectedID {
		return platformContractError(operation, "LinkedIn returned a missing or mismatched Campaign ID")
	}
	if value.Account != client.accountURN() {
		return platformContractError(operation, "LinkedIn returned a Campaign owned by another Ad Account")
	}
	if value.CampaignGroup != "" && !validNumericURN(value.CampaignGroup, campaignGroupURNPrefix) ||
		expectedGroupID != "" && value.CampaignGroup != campaignGroupURNPrefix+expectedGroupID {
		return platformContractError(operation, "LinkedIn returned an invalid or mismatched Campaign Group")
	}
	return nil
}

func campaignCurrenciesMatch(input CreateCampaignRequest) bool {
	currency := input.DailyBudget.CurrencyCode
	return input.UnitCost.CurrencyCode == currency && (input.TotalBudget == nil || input.TotalBudget.CurrencyCode == currency)
}
