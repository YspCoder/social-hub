package ads

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type campaignPage struct {
	Items    []Campaign `json:"items"`
	Bookmark string     `json:"bookmark"`
}

type campaignMutationResource struct {
	AdAccountID                string        `json:"ad_account_id"`
	ID                         string        `json:"id,omitempty"`
	Name                       string        `json:"name,omitempty"`
	Objective                  ObjectiveType `json:"objective_type,omitempty"`
	Status                     EntityStatus  `json:"status,omitempty"`
	DailySpendCap              *int64        `json:"daily_spend_cap,omitempty"`
	LifetimeSpendCap           *int64        `json:"lifetime_spend_cap,omitempty"`
	StartTime                  *int64        `json:"start_time,omitempty"`
	EndTime                    *int64        `json:"end_time,omitempty"`
	CampaignBudgetOptimization *bool         `json:"is_campaign_budget_optimization,omitempty"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (socialhub.Page[Campaign], error) {
	const operation = "campaigns_list"
	if !validIDs(input.IDs) || !validStatuses(input.Statuses) || !validPage(input.Cursor, input.MaxResults) {
		return socialhub.Page[Campaign]{}, invalidArgument(operation, "Campaign IDs, statuses, bookmark, or page size are invalid")
	}
	query := listQuery(input.Cursor, input.MaxResults)
	addQueryValues(query, "campaign_ids", input.IDs)
	addStatusValues(query, input.Statuses)
	var response campaignPage
	if _, err := client.getJSON(ctx, operation, client.resourcePath("campaigns"), query, &response, options...); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	for index := range response.Items {
		if err := client.validateCampaign(operation, &response.Items[index], ""); err != nil {
			return socialhub.Page[Campaign]{}, err
		}
	}
	return toPage(response.Items, response.Bookmark), nil
}

func (client *Client) GetCampaign(ctx context.Context, id string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validID(id) {
		return nil, invalidArgument(operation, "Campaign ID is invalid")
	}
	var response Campaign
	if _, err := client.getJSON(ctx, operation, client.resourcePath("campaigns/"+id), nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaign(operation, &response, id); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validText(input.Name, 255) || !validObjective(input.Objective) ||
		!validCampaignBudget(input.CampaignBudgetOptimization, input.DailySpendCap, input.LifetimeSpendCap) ||
		!validSchedule(input.StartTime, input.EndTime) {
		return nil, invalidArgument(operation, "Campaign name, objective, budget, or schedule is invalid")
	}
	cbo := input.CampaignBudgetOptimization
	resource := campaignMutationResource{
		AdAccountID: client.adAccountID, Name: input.Name, Objective: input.Objective,
		Status: StatusPaused, CampaignBudgetOptimization: &cbo,
	}
	setPositiveInt64(&resource.DailySpendCap, input.DailySpendCap)
	setPositiveInt64(&resource.LifetimeSpendCap, input.LifetimeSpendCap)
	setPositiveInt64(&resource.StartTime, input.StartTime)
	setPositiveInt64(&resource.EndTime, input.EndTime)
	return client.mutateCampaign(ctx, operation, resource, "", options...)
}

func (client *Client) UpdateCampaign(ctx context.Context, id string, input UpdateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_update"
	if !validID(id) || input.Name == nil && input.DailySpendCap == nil && input.LifetimeSpendCap == nil && input.StartTime == nil && input.EndTime == nil {
		return nil, invalidArgument(operation, "Campaign ID and at least one update are required")
	}
	if input.Name != nil && !validText(*input.Name, 255) || !validUpdateSchedule(input.StartTime, input.EndTime) {
		return nil, invalidArgument(operation, "Campaign name or schedule is invalid")
	}
	if input.DailySpendCap != nil && *input.DailySpendCap < 0 || input.LifetimeSpendCap != nil && *input.LifetimeSpendCap < 0 ||
		input.DailySpendCap != nil && input.LifetimeSpendCap != nil && *input.DailySpendCap > 0 && *input.LifetimeSpendCap > 0 {
		return nil, invalidArgument(operation, "Campaign spend caps must be non-negative and mutually exclusive")
	}
	resource := campaignMutationResource{
		AdAccountID: client.adAccountID, ID: id,
		DailySpendCap: input.DailySpendCap, LifetimeSpendCap: input.LifetimeSpendCap,
		StartTime: input.StartTime, EndTime: input.EndTime,
	}
	if input.Name != nil {
		resource.Name = *input.Name
	}
	return client.mutateCampaign(ctx, operation, resource, id, options...)
}

func (client *Client) SetCampaignStatus(ctx context.Context, id string, status EntityStatus, options ...socialhub.CallOption) (*Campaign, error) {
	if !validID(id) || !validMutationStatus(status) {
		return nil, invalidArgument("campaign_status", "Campaign ID and ACTIVE or PAUSED status are required")
	}
	return client.mutateCampaign(ctx, "campaign_status", campaignMutationResource{AdAccountID: client.adAccountID, ID: id, Status: status}, id, options...)
}

func (client *Client) ArchiveCampaign(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if !validID(id) {
		return invalidArgument("campaign_archive", "Campaign ID is invalid")
	}
	_, err := client.mutateCampaign(ctx, "campaign_archive", campaignMutationResource{AdAccountID: client.adAccountID, ID: id, Status: StatusArchived}, id, options...)
	return err
}

func (client *Client) mutateCampaign(ctx context.Context, operation string, resource campaignMutationResource, expected string, options ...socialhub.CallOption) (*Campaign, error) {
	var response batchResponse[Campaign]
	method := http.MethodPatch
	if expected == "" {
		method = http.MethodPost
	}
	metadata, err := client.writeJSON(ctx, operation, method, client.resourcePath("campaigns"), []campaignMutationResource{resource}, &response, options...)
	if err != nil {
		return nil, err
	}
	return requireBatchResult(operation, response, metadata, func(campaign *Campaign) error {
		return client.validateCampaign(operation, campaign, expected)
	})
}

func (client *Client) validateCampaign(operation string, campaign *Campaign, expected string) error {
	if !validID(campaign.ID) || expected != "" && campaign.ID != expected {
		return platformContractError(operation, "Pinterest returned a missing or mismatched Campaign ID")
	}
	if campaign.AdAccountID != "" && campaign.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Pinterest returned a Campaign owned by another Ad Account")
	}
	if campaign.AdAccountID == "" {
		campaign.AdAccountID = client.adAccountID
	}
	return nil
}

func (client *Client) resourcePath(resource string) string {
	return "/ad_accounts/" + client.adAccountID + "/" + resource
}

func addQueryValues(query url.Values, key string, values []string) {
	for _, value := range values {
		query.Add(key, value)
	}
}

func addStatusValues(query url.Values, statuses []EntityStatus) {
	for _, status := range statuses {
		query.Add("entity_statuses", string(status))
	}
}

func setPositiveInt64(target **int64, value int64) {
	if value > 0 {
		copy := value
		*target = &copy
	}
}
