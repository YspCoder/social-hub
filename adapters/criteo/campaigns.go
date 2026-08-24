package criteo

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

const campaignsPath = "/2026-01/marketing-solutions/campaigns"

func (client *Client) GetCampaign(ctx context.Context, campaignID string, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_get"
	if !validID(campaignID) {
		return Campaign{}, invalidArgument(operation, "campaign ID must be a positive string-encoded integer")
	}
	var response apiEnvelope[entityResource[campaignAttributes]]
	if err := client.getJSON(ctx, operation, campaignsPath+"/"+campaignID, nil, &response, options...); err != nil {
		return Campaign{}, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return Campaign{}, err
	}
	campaign, err := campaignFromResource(operation, response.Data)
	if err != nil {
		return Campaign{}, err
	}
	if campaign.ID != campaignID {
		return Campaign{}, platformContractError(operation, "Criteo returned a different campaign")
	}
	if campaign.AdvertiserID != client.advertiserID {
		return Campaign{}, ownershipError(operation)
	}
	return campaign, nil
}

func (client *Client) SearchCampaigns(ctx context.Context, input CampaignSearchRequest, options ...socialhub.CallOption) ([]Campaign, error) {
	const operation = "campaign_search"
	if !validIDs(input.CampaignIDs, 1000) {
		return nil, invalidArgument(operation, "campaign IDs must be unique positive string-encoded integers")
	}
	var request campaignSearchEnvelope
	request.Filters.AdvertiserIDs = []string{client.advertiserID}
	request.Filters.CampaignIDs = append([]string(nil), input.CampaignIDs...)
	var response apiEnvelope[[]entityResource[campaignAttributes]]
	if err := client.postJSON(ctx, operation, campaignsPath+"/search", request, &response, options...); err != nil {
		return nil, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return nil, err
	}
	result := make([]Campaign, 0, len(response.Data))
	seen := make(map[string]struct{}, len(response.Data))
	for _, resource := range response.Data {
		campaign, err := campaignFromResource(operation, resource)
		if err != nil {
			return nil, err
		}
		if campaign.AdvertiserID != client.advertiserID {
			return nil, ownershipError(operation)
		}
		if _, exists := seen[campaign.ID]; exists {
			return nil, platformContractError(operation, "Criteo returned duplicate campaigns")
		}
		seen[campaign.ID] = struct{}{}
		result = append(result, campaign)
	}
	return result, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_create"
	if !validText(input.Name, 1024) || !validCampaignGoal(input.Goal) || !validSpendLimit(input.SpendLimit) ||
		!validCreateBudgetAutomation(input.BudgetAutomation) {
		return Campaign{}, invalidArgument(operation, "name, goal, spend limit, or budget automation is invalid")
	}
	attributes := createCampaignAttributes{
		AdvertiserID: client.advertiserID, Name: input.Name, Goal: campaignGoalWire(input.Goal),
		SpendLimit: createCampaignSpendLimitWire{
			Type: input.SpendLimit.Type, Amount: input.SpendLimit.Amount, Renewal: input.SpendLimit.Renewal,
		},
	}
	if input.SpendLimit.Type == SpendLimitUncapped {
		attributes.SpendLimit.Renewal = ""
	}
	if input.BudgetAutomation != nil {
		attributes.BudgetAutomation = &budgetAutomationWrite{Enabled: input.BudgetAutomation.Enabled}
		attributes.BudgetAutomation.BudgetConfiguration.AdSetObjectives = input.BudgetAutomation.Objective
	}
	request := campaignWriteEnvelope{Data: entityResource[createCampaignAttributes]{Type: "Campaign", Attributes: attributes}}
	var response apiEnvelope[entityResource[campaignAttributes]]
	if err := client.postJSON(ctx, operation, campaignsPath, request, &response, options...); err != nil {
		return Campaign{}, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return Campaign{}, err
	}
	campaign, err := campaignFromResource(operation, response.Data)
	if err != nil {
		return Campaign{}, err
	}
	if campaign.AdvertiserID != client.advertiserID || campaign.Name != input.Name || campaign.Goal != input.Goal {
		return Campaign{}, platformContractError(operation, "Criteo returned a campaign that does not match the create request")
	}
	return campaign, nil
}

func (client *Client) UpdateCampaign(ctx context.Context, campaignID string, input UpdateCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "campaign_update"
	if !validID(campaignID) || !validCampaignPatch(input) {
		return Campaign{}, invalidArgument(operation, "campaign ID or patch fields are invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return Campaign{}, withOperation(err, operation)
	}
	attributes := campaignPatchAttributes{SpendLimit: input.SpendLimit}
	if input.BudgetAutomation != nil {
		attributes.BudgetAutomation = &budgetAutomationPatch{Enabled: input.BudgetAutomation.Enabled}
		if input.BudgetAutomation.Objective != "" {
			attributes.BudgetAutomation.BudgetConfiguration = &struct {
				AdSetObjectives BudgetAutomationObjective `json:"adSetObjectives"`
			}{AdSetObjectives: input.BudgetAutomation.Objective}
		}
	}
	if input.ScheduledSpendLimit != nil {
		wire := &scheduledSpendPatchWire{
			Creations: append([]ScheduledSpendLimitCreation(nil), input.ScheduledSpendLimit.Creations...),
			Updates:   append([]ScheduledSpendLimitUpdate(nil), input.ScheduledSpendLimit.Updates...),
		}
		for _, id := range input.ScheduledSpendLimit.Deletions {
			wire.Deletions = append(wire.Deletions, struct {
				ID string `json:"id"`
			}{ID: id})
		}
		attributes.ScheduledSpendLimit = wire
	}
	request := campaignPatchEnvelope{Data: []entityResource[campaignPatchAttributes]{{
		Type: "Campaign", ID: campaignID, Attributes: attributes,
	}}}
	var response apiEnvelope[[]idResource]
	if err := client.patchJSON(ctx, operation, campaignsPath, request, &response, options...); err != nil {
		return Campaign{}, err
	}
	if err := checkProblems(operation, response.Errors); err != nil {
		return Campaign{}, err
	}
	if err := validateIDResult(operation, response.Data, campaignID, "Campaign"); err != nil {
		return Campaign{}, err
	}
	updated, err := client.GetCampaign(ctx, campaignID, options...)
	return updated, withOperation(err, operation)
}

func campaignFromResource(operation string, resource entityResource[campaignAttributes]) (Campaign, error) {
	if resource.Type != "" && !strings.EqualFold(resource.Type, "Campaign") {
		return Campaign{}, platformContractError(operation, "Criteo returned an invalid campaign resource type")
	}
	id := resource.ID
	if id == "" {
		id = resource.Attributes.ID
	}
	if resource.ID != "" && resource.Attributes.ID != "" && resource.ID != resource.Attributes.ID {
		return Campaign{}, platformContractError(operation, "Criteo returned inconsistent campaign IDs")
	}
	goal := CampaignGoal(strings.ToLower(string(resource.Attributes.Goal)))
	if !validID(id) || !validID(resource.Attributes.AdvertiserID) || !validText(resource.Attributes.Name, 1024) ||
		!validCampaignGoal(goal) {
		return Campaign{}, platformContractError(operation, "Criteo returned an invalid campaign")
	}
	result := Campaign{
		ID: id, AdvertiserID: resource.Attributes.AdvertiserID, Name: resource.Attributes.Name,
		Goal: goal, SpendLimit: resource.Attributes.SpendLimit,
		ScheduledSpendLimits: append([]ScheduledSpendLimit(nil), resource.Attributes.ScheduledSpendLimits...),
	}
	if resource.Attributes.BudgetAutomation != nil {
		result.BudgetAutomation = &CampaignBudgetAutomation{
			Enabled:                    resource.Attributes.BudgetAutomation.Enabled,
			AdSetOptimizationObjective: resource.Attributes.BudgetAutomation.AutomatedBudgetConfiguration.AdSetOptimizationObjective,
		}
	}
	return result, nil
}

func campaignGoalWire(value CampaignGoal) string {
	if value == GoalAcquisition {
		return "Acquisition"
	}
	return "Retention"
}

func validCreateBudgetAutomation(value *CreateBudgetAutomation) bool {
	return value == nil || validAutomationObjective(value.Objective)
}

func validAutomationObjective(value BudgetAutomationObjective) bool {
	switch value {
	case AutomationConversions, AutomationRevenue, AutomationVisits, AutomationVideoViews:
		return true
	default:
		return false
	}
}

func validCampaignPatch(input UpdateCampaignRequest) bool {
	if input.SpendLimit == nil && input.BudgetAutomation == nil && input.ScheduledSpendLimit == nil {
		return false
	}
	if input.SpendLimit != nil {
		value := input.SpendLimit
		if value.Type == "" && value.Amount == nil && value.Renewal == "" {
			return false
		}
		if value.Type != "" && value.Type != SpendLimitCapped && value.Type != SpendLimitUncapped {
			return false
		}
		if value.Amount != nil && value.Amount.Value != nil && !validPositive(*value.Amount.Value) {
			return false
		}
		if value.Renewal != "" && value.Renewal != RenewalUndefined && value.Renewal != RenewalDaily &&
			value.Renewal != RenewalMonthly && value.Renewal != RenewalLifetime {
			return false
		}
	}
	if input.BudgetAutomation != nil {
		if input.BudgetAutomation.Enabled == nil && input.BudgetAutomation.Objective == "" {
			return false
		}
		if input.BudgetAutomation.Objective != "" && !validAutomationObjective(input.BudgetAutomation.Objective) {
			return false
		}
	}
	if input.ScheduledSpendLimit != nil {
		value := input.ScheduledSpendLimit
		if len(value.Creations)+len(value.Updates)+len(value.Deletions) == 0 ||
			len(value.Creations)+len(value.Updates)+len(value.Deletions) > 50 || !validIDs(value.Deletions, 50) {
			return false
		}
		for _, creation := range value.Creations {
			if !validScheduledLimit(creation.Type, creation.Amount, creation.Renewal, creation.StartDate) {
				return false
			}
		}
		for _, update := range value.Updates {
			if !validID(update.ID) || !validScheduledLimit(update.Type, update.Amount, update.Renewal, update.StartDate) {
				return false
			}
		}
	}
	return true
}

func validScheduledLimit(limitType SpendLimitType, amount *NullableFloat, renewal SpendLimitRenewal, date string) bool {
	if _, ok := parseReportDate(date); !ok {
		return false
	}
	if limitType == SpendLimitCapped {
		return amount != nil && amount.Value != nil && validPositive(*amount.Value) &&
			(renewal == RenewalDaily || renewal == RenewalMonthly || renewal == RenewalLifetime)
	}
	return limitType == SpendLimitUncapped && (amount == nil || amount.Value == nil) &&
		(renewal == "" || renewal == RenewalUndefined)
}
