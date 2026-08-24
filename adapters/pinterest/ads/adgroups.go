package ads

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

type adGroupPage struct {
	Items    []AdGroup `json:"items"`
	Bookmark string    `json:"bookmark"`
}

type adGroupMutationResource struct {
	ID                       string             `json:"id,omitempty"`
	CampaignID               string             `json:"campaign_id,omitempty"`
	Name                     string             `json:"name,omitempty"`
	Status                   EntityStatus       `json:"status,omitempty"`
	BillableEvent            BillableEvent      `json:"billable_event,omitempty"`
	BudgetType               BudgetType         `json:"budget_type,omitempty"`
	BudgetInMicroCurrency    *int64             `json:"budget_in_micro_currency,omitempty"`
	BidInMicroCurrency       *int64             `json:"bid_in_micro_currency,omitempty"`
	BidStrategy              BidStrategyType    `json:"bid_strategy_type,omitempty"`
	Pacing                   PacingDeliveryType `json:"pacing_delivery_type,omitempty"`
	Placement                PlacementGroup     `json:"placement_group,omitempty"`
	Targeting                TargetingSpec      `json:"targeting_spec,omitempty"`
	OptimizationGoalMetadata map[string]any     `json:"optimization_goal_metadata,omitempty"`
	StartTime                *int64             `json:"start_time,omitempty"`
	EndTime                  *int64             `json:"end_time,omitempty"`
}

func (client *Client) ListAdGroups(ctx context.Context, input ListAdGroupsRequest, options ...socialhub.CallOption) (socialhub.Page[AdGroup], error) {
	const operation = "ad_groups_list"
	if !validIDs(input.IDs) || !validIDs(input.CampaignIDs) || !validStatuses(input.Statuses) || !validPage(input.Cursor, input.MaxResults) {
		return socialhub.Page[AdGroup]{}, invalidArgument(operation, "Ad Group filters, bookmark, or page size are invalid")
	}
	query := listQuery(input.Cursor, input.MaxResults)
	addQueryValues(query, "ad_group_ids", input.IDs)
	addQueryValues(query, "campaign_ids", input.CampaignIDs)
	addStatusValues(query, input.Statuses)
	var response adGroupPage
	if _, err := client.getJSON(ctx, operation, client.resourcePath("ad_groups"), query, &response, options...); err != nil {
		return socialhub.Page[AdGroup]{}, err
	}
	for index := range response.Items {
		if err := client.validateAdGroup(operation, &response.Items[index], ""); err != nil {
			return socialhub.Page[AdGroup]{}, err
		}
	}
	return toPage(response.Items, response.Bookmark), nil
}

func (client *Client) GetAdGroup(ctx context.Context, id string, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_get"
	if !validID(id) {
		return nil, invalidArgument(operation, "Ad Group ID is invalid")
	}
	var response AdGroup
	if _, err := client.getJSON(ctx, operation, client.resourcePath("ad_groups/"+id), nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &response, id); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_create"
	if !validID(input.CampaignID) || !validText(input.Name, 255) || !validBillableEvent(input.BillableEvent) ||
		input.BudgetInMicroCurrency < 0 || input.BidInMicroCurrency < 0 || !validSchedule(input.StartTime, input.EndTime) ||
		!validTargeting(input.Targeting) || !validJSONMap(input.OptimizationGoalMetadata) || !validBidStrategy(input.BidStrategy) {
		return nil, invalidArgument(operation, "Ad Group identity, bid, schedule, targeting, or optimization metadata is invalid")
	}
	budgetType := input.BudgetType
	if budgetType == "" {
		budgetType = BudgetDaily
	}
	pacing := input.Pacing
	if pacing == "" {
		pacing = PacingStandard
	}
	placement := input.Placement
	if placement == "" {
		placement = PlacementAll
	}
	if !validBudgetType(budgetType) || !validPacing(pacing) || !validPlacement(placement) {
		return nil, invalidArgument(operation, "Ad Group budget, pacing, or placement enum is invalid")
	}
	resource := adGroupMutationResource{
		CampaignID: input.CampaignID, Name: input.Name, Status: StatusPaused,
		BillableEvent: input.BillableEvent, BudgetType: budgetType, BidStrategy: input.BidStrategy,
		Pacing: pacing, Placement: placement, Targeting: copyTargeting(input.Targeting),
		OptimizationGoalMetadata: input.OptimizationGoalMetadata,
	}
	setPositiveInt64(&resource.BudgetInMicroCurrency, input.BudgetInMicroCurrency)
	setPositiveInt64(&resource.BidInMicroCurrency, input.BidInMicroCurrency)
	setPositiveInt64(&resource.StartTime, input.StartTime)
	setPositiveInt64(&resource.EndTime, input.EndTime)
	return client.mutateAdGroup(ctx, operation, http.MethodPost, resource, "", options...)
}

func (client *Client) UpdateAdGroup(ctx context.Context, id string, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_update"
	if !validID(id) || input.Name == nil && input.BudgetInMicroCurrency == nil && input.BidInMicroCurrency == nil &&
		input.BidStrategy == nil && input.Pacing == nil && input.Placement == nil && input.Targeting == nil &&
		input.OptimizationGoalMetadata == nil && input.EndTime == nil {
		return nil, invalidArgument(operation, "Ad Group ID and at least one update are required")
	}
	if input.Name != nil && !validText(*input.Name, 255) || input.BudgetInMicroCurrency != nil && *input.BudgetInMicroCurrency < 0 ||
		input.BidInMicroCurrency != nil && *input.BidInMicroCurrency < 0 || input.EndTime != nil && *input.EndTime < -1 ||
		input.BidStrategy != nil && !validBidStrategy(*input.BidStrategy) || input.Pacing != nil && !validPacing(*input.Pacing) ||
		input.Placement != nil && !validPlacement(*input.Placement) || input.Targeting != nil && !validTargeting(input.Targeting) ||
		!validJSONMap(input.OptimizationGoalMetadata) {
		return nil, invalidArgument(operation, "one or more Ad Group update fields are invalid")
	}
	resource := adGroupMutationResource{
		ID: id, BudgetInMicroCurrency: input.BudgetInMicroCurrency, BidInMicroCurrency: input.BidInMicroCurrency,
		Targeting: copyTargeting(input.Targeting), OptimizationGoalMetadata: input.OptimizationGoalMetadata,
		EndTime: input.EndTime,
	}
	if input.Name != nil {
		resource.Name = *input.Name
	}
	if input.BidStrategy != nil {
		resource.BidStrategy = *input.BidStrategy
	}
	if input.Pacing != nil {
		resource.Pacing = *input.Pacing
	}
	if input.Placement != nil {
		resource.Placement = *input.Placement
	}
	return client.mutateAdGroup(ctx, operation, http.MethodPatch, resource, id, options...)
}

func (client *Client) SetAdGroupStatus(ctx context.Context, id string, status EntityStatus, options ...socialhub.CallOption) (*AdGroup, error) {
	if !validID(id) || !validMutationStatus(status) {
		return nil, invalidArgument("ad_group_status", "Ad Group ID and ACTIVE or PAUSED status are required")
	}
	return client.mutateAdGroup(ctx, "ad_group_status", http.MethodPatch, adGroupMutationResource{ID: id, Status: status}, id, options...)
}

func (client *Client) ArchiveAdGroup(ctx context.Context, id string, options ...socialhub.CallOption) error {
	if !validID(id) {
		return invalidArgument("ad_group_archive", "Ad Group ID is invalid")
	}
	_, err := client.mutateAdGroup(ctx, "ad_group_archive", http.MethodPatch, adGroupMutationResource{ID: id, Status: StatusArchived}, id, options...)
	return err
}

func (client *Client) mutateAdGroup(ctx context.Context, operation, method string, resource adGroupMutationResource, expected string, options ...socialhub.CallOption) (*AdGroup, error) {
	var response batchResponse[AdGroup]
	metadata, err := client.writeJSON(ctx, operation, method, client.resourcePath("ad_groups"), []adGroupMutationResource{resource}, &response, options...)
	if err != nil {
		return nil, err
	}
	return requireBatchResult(operation, response, metadata, func(adGroup *AdGroup) error {
		return client.validateAdGroup(operation, adGroup, expected)
	})
}

func (client *Client) validateAdGroup(operation string, adGroup *AdGroup, expected string) error {
	if !validID(adGroup.ID) || expected != "" && adGroup.ID != expected || !validID(adGroup.CampaignID) {
		return platformContractError(operation, "Pinterest returned an invalid or mismatched Ad Group or Campaign ID")
	}
	if adGroup.AdAccountID != "" && adGroup.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Pinterest returned an Ad Group owned by another Ad Account")
	}
	if adGroup.AdAccountID == "" {
		adGroup.AdAccountID = client.adAccountID
	}
	return nil
}

func copyTargeting(input TargetingSpec) TargetingSpec {
	if input == nil {
		return nil
	}
	result := make(TargetingSpec, len(input))
	for key, values := range input {
		result[key] = append([]string(nil), values...)
	}
	return result
}
