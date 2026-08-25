package yahoosearchads

import (
	"context"

	"social-hub/pkg/socialhub"
)

const adGroupServicePath = "AdGroupService"

type adGroupSelectorRequest struct {
	AccountID     int64        `json:"accountId"`
	CampaignIDs   []int64      `json:"campaignIds,omitempty"`
	AdGroupIDs    []int64      `json:"adGroupIds,omitempty"`
	UserStatuses  []UserStatus `json:"userStatuses,omitempty"`
	StartIndex    int32        `json:"startIndex,omitempty"`
	NumberResults int32        `json:"numberResults,omitempty"`
}

type adGroupOperation struct {
	AccountID int64     `json:"accountId"`
	Operand   []AdGroup `json:"operand"`
}

func (client *Client) ListAdGroups(ctx context.Context, input AdGroupSelector, options ...socialhub.CallOption) (Page[AdGroup], error) {
	const operation = "adgroup_list"
	if !validAdGroupSelector(input) {
		return Page[AdGroup]{}, invalidArgument(operation, "campaign IDs, ad group IDs, statuses, or pagination are invalid")
	}
	request := adGroupSelectorRequest{
		AccountID: client.advertiserAccountID, CampaignIDs: input.CampaignIDs, AdGroupIDs: input.AdGroupIDs,
		UserStatuses: input.UserStatuses, StartIndex: input.StartIndex, NumberResults: input.NumberResults,
	}
	return postPage(ctx, client, operation, adGroupServicePath+"/get", request, input.PageRequest,
		MaximumPageSize, adGroupEntity, func(value *AdGroup) error {
			return client.validateAdGroup(operation, value, 0, 0)
		}, options...)
}

func (client *Client) GetAdGroup(ctx context.Context, campaignID, adGroupID int64, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_get"
	if campaignID <= 0 || adGroupID <= 0 {
		return nil, invalidArgument(operation, "campaign ID and ad group ID must be positive")
	}
	page, err := client.ListAdGroups(ctx, AdGroupSelector{
		CampaignIDs: []int64{campaignID}, AdGroupIDs: []int64{adGroupID},
		PageRequest: PageRequest{StartIndex: 1, NumberResults: 1},
	}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "ad group was not returned")
	}
	if len(page.Items) != 1 || page.Items[0].CampaignID != campaignID || page.Items[0].AdGroupID != adGroupID {
		return nil, platformContractError(operation, "LINE Yahoo returned a different ad group")
	}
	return &page.Items[0], nil
}

func (client *Client) CreateAdGroups(ctx context.Context, campaignID int64, inputs []AdGroupAdd, options ...socialhub.CallOption) (MutationResult[AdGroup], error) {
	const operation = "adgroup_create"
	if campaignID <= 0 || len(inputs) == 0 || len(inputs) > MaximumMutationBatch {
		return MutationResult[AdGroup]{}, invalidArgument(operation, "campaign ID and 1-2000 ad groups are required")
	}
	operands := make([]AdGroup, 0, len(inputs))
	for _, input := range inputs {
		if !validAdGroupAdd(input) {
			return MutationResult[AdGroup]{}, invalidArgument(operation, "paused CPC ad group name or bid is invalid")
		}
		cpc := input.CPC
		operands = append(operands, AdGroup{
			CampaignID: campaignID, AdGroupName: input.Name, UserStatus: StatusPaused,
			BiddingStrategyConfiguration: &AdGroupBiddingStrategy{
				BiddingScheme: &AdGroupBiddingScheme{CPC: &AdGroupCPCScheme{CPC: &cpc}},
			},
		})
	}
	return postMutation(ctx, client, operation, adGroupServicePath+"/add",
		adGroupOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adGroupEntity, func(value *AdGroup) error {
			return client.validateAdGroupMutation(operation, value, campaignID)
		}, options...)
}

func (client *Client) UpdateAdGroups(ctx context.Context, campaignID int64, inputs []AdGroupUpdate, options ...socialhub.CallOption) (MutationResult[AdGroup], error) {
	const operation = "adgroup_update"
	if campaignID <= 0 || len(inputs) == 0 || len(inputs) > MaximumMutationBatch {
		return MutationResult[AdGroup]{}, invalidArgument(operation, "campaign ID and 1-2000 ad group updates are required")
	}
	seen := make(map[int64]struct{}, len(inputs))
	operands := make([]AdGroup, 0, len(inputs))
	for _, input := range inputs {
		if !validAdGroupUpdate(input) {
			return MutationResult[AdGroup]{}, invalidArgument(operation, "ad group update is invalid")
		}
		if _, exists := seen[input.ID]; exists {
			return MutationResult[AdGroup]{}, invalidArgument(operation, "ad group IDs must be unique")
		}
		seen[input.ID] = struct{}{}
		operand := AdGroup{CampaignID: campaignID, AdGroupID: input.ID}
		if input.Name != nil {
			operand.AdGroupName = *input.Name
		}
		if input.CPC != nil {
			cpc := *input.CPC
			operand.BiddingStrategyConfiguration = &AdGroupBiddingStrategy{
				BiddingScheme: &AdGroupBiddingScheme{CPC: &AdGroupCPCScheme{CPC: &cpc}},
			}
		}
		operands = append(operands, operand)
	}
	return postMutation(ctx, client, operation, adGroupServicePath+"/set",
		adGroupOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adGroupEntity, func(value *AdGroup) error {
			return client.validateAdGroupMutation(operation, value, campaignID)
		}, options...)
}

func (client *Client) SetAdGroupsEnabled(ctx context.Context, campaignID int64, ids []int64, enabled bool, options ...socialhub.CallOption) (MutationResult[AdGroup], error) {
	const operation = "adgroup_set_enabled"
	if campaignID <= 0 || !validIDs(ids, MaximumMutationBatch, false) {
		return MutationResult[AdGroup]{}, invalidArgument(operation, "campaign ID and 1-2000 unique ad group IDs are required")
	}
	status := StatusPaused
	if enabled {
		status = StatusActive
	}
	operands := make([]AdGroup, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, AdGroup{CampaignID: campaignID, AdGroupID: id, UserStatus: status})
	}
	return postMutation(ctx, client, operation, adGroupServicePath+"/set",
		adGroupOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adGroupEntity, func(value *AdGroup) error {
			return client.validateAdGroupMutation(operation, value, campaignID)
		}, options...)
}

func (client *Client) DeleteAdGroups(ctx context.Context, campaignID int64, ids []int64, options ...socialhub.CallOption) (MutationResult[AdGroup], error) {
	const operation = "adgroup_delete"
	if campaignID <= 0 || !validIDs(ids, MaximumMutationBatch, false) {
		return MutationResult[AdGroup]{}, invalidArgument(operation, "campaign ID and 1-2000 unique ad group IDs are required")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return MutationResult[AdGroup]{}, err
	}
	if err := client.requirePausedAdGroups(ctx, operation, campaignID, ids, prepared...); err != nil {
		return MutationResult[AdGroup]{}, err
	}
	operands := make([]AdGroup, 0, len(ids))
	for _, id := range ids {
		operands = append(operands, AdGroup{CampaignID: campaignID, AdGroupID: id})
	}
	return postMutation(ctx, client, operation, adGroupServicePath+"/remove",
		adGroupOperation{AccountID: client.advertiserAccountID, Operand: operands}, len(operands),
		adGroupEntity, func(value *AdGroup) error {
			return client.validateAdGroupMutation(operation, value, campaignID)
		}, prepared...)
}

func (client *Client) requirePausedAdGroups(ctx context.Context, operation string, campaignID int64, ids []int64, options ...socialhub.CallOption) error {
	expected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		expected[id] = struct{}{}
	}
	for start := 0; start < len(ids); start += MaximumSelectorIDs {
		end := start + MaximumSelectorIDs
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		page, err := client.ListAdGroups(ctx, AdGroupSelector{
			CampaignIDs: []int64{campaignID}, AdGroupIDs: chunk,
			PageRequest: PageRequest{StartIndex: 1, NumberResults: int32(len(chunk))},
		}, options...)
		if err != nil {
			return withOperation(err, operation)
		}
		if len(page.Items) != len(chunk) {
			return notFound(operation, "one or more ad groups were not returned before delete")
		}
		for _, adGroup := range page.Items {
			if adGroup.CampaignID != campaignID {
				return platformContractError(operation, "LINE Yahoo returned an ad group from another campaign")
			}
			if _, exists := expected[adGroup.AdGroupID]; !exists {
				return platformContractError(operation, "LINE Yahoo returned a duplicate ad group or one outside the delete selection")
			}
			delete(expected, adGroup.AdGroupID)
			if adGroup.UserStatus != StatusPaused {
				return invalidArgument(operation, "ad groups must be PAUSED before delete")
			}
		}
	}
	if len(expected) != 0 {
		return platformContractError(operation, "LINE Yahoo omitted one or more ad groups from the delete preflight")
	}
	return nil
}

func (client *Client) validateAdGroup(operation string, value *AdGroup, expectedCampaignID, expectedAdGroupID int64) error {
	if value == nil || value.AccountID != client.advertiserAccountID || value.CampaignID <= 0 || value.AdGroupID <= 0 ||
		!validText(value.AdGroupName, 50) || (value.UserStatus != StatusActive && value.UserStatus != StatusPaused) {
		return platformContractError(operation, "LINE Yahoo returned an invalid ad group")
	}
	if expectedCampaignID > 0 && value.CampaignID != expectedCampaignID ||
		expectedAdGroupID > 0 && value.AdGroupID != expectedAdGroupID {
		return platformContractError(operation, "ad group parent or ID did not match the request")
	}
	return nil
}

func (client *Client) validateAdGroupMutation(operation string, value *AdGroup, expectedCampaignID int64) error {
	if value == nil || value.AdGroupID <= 0 || value.CampaignID != 0 && value.CampaignID != expectedCampaignID ||
		value.AccountID != 0 && value.AccountID != client.advertiserAccountID {
		return platformContractError(operation, "LINE Yahoo returned an invalid ad group mutation value")
	}
	return nil
}

var _ AdGroupWorkflow = (*Client)(nil)
