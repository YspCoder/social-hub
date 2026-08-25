package yandexdirect

import (
	"context"
	"sort"

	"social-hub/pkg/socialhub"
)

var adGroupFields = []string{
	"Id", "Name", "CampaignId", "RegionIds", "RestrictedRegionIds", "NegativeKeywords",
	"NegativeKeywordSharedSetIds", "TrackingParams", "Status", "ServingStatus", "Type",
}

func (client *Client) ListAdGroups(ctx context.Context, input ListAdGroupsRequest, options ...socialhub.CallOption) (Page[AdGroup], error) {
	const operation = "adgroup_list"
	if !validAdGroupSelection(input.Selection) || !validPage(input.Page) {
		return Page[AdGroup]{}, invalidArgument(operation, "Ad Group selection or page is invalid")
	}
	params := struct {
		SelectionCriteria AdGroupSelection `json:"SelectionCriteria"`
		FieldNames        []string         `json:"FieldNames"`
		Page              *PageRequest     `json:"Page,omitempty"`
	}{SelectionCriteria: input.Selection, FieldNames: adGroupFields, Page: pagePointer(input.Page)}
	var response struct {
		AdGroups  []AdGroup `json:"AdGroups"`
		LimitedBy *int64    `json:"LimitedBy"`
	}
	metadata, err := client.rpc(ctx, operation, "adgroups", "get", params, &response, false, options...)
	if err != nil {
		return Page[AdGroup]{}, err
	}
	if len(response.AdGroups) > maximumPageItems(input.Page) || response.LimitedBy != nil && *response.LimitedBy <= input.Page.Offset {
		return Page[AdGroup]{}, platformContractError(operation, "Yandex returned invalid Ad Group pagination")
	}
	for index := range response.AdGroups {
		if err := validateAdGroup(operation, &response.AdGroups[index], 0); err != nil {
			return Page[AdGroup]{}, err
		}
	}
	return Page[AdGroup]{Items: response.AdGroups, LimitedBy: response.LimitedBy, Metadata: metadata}, nil
}

func (client *Client) GetAdGroup(ctx context.Context, id int64, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "Ad Group ID must be positive")
	}
	page, err := client.ListAdGroups(ctx, ListAdGroupsRequest{Selection: AdGroupSelection{IDs: []int64{id}}}, options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if len(page.Items) == 0 {
		return nil, notFound(operation, "Ad Group was not returned")
	}
	if len(page.Items) != 1 || page.Items[0].ID != id {
		return nil, platformContractError(operation, "Yandex returned a different Ad Group")
	}
	return &page.Items[0], nil
}

func (client *Client) CreateAdGroups(ctx context.Context, campaignID int64, inputs []AdGroupAdd, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "adgroup_create"
	if campaignID <= 0 || len(inputs) == 0 || len(inputs) > MaximumAdGroupMutationBatch {
		return BatchResult{}, invalidArgument(operation, "positive Campaign ID and 1-1000 Ad Groups are required")
	}
	for _, input := range inputs {
		if !validAdGroupAdd(input) {
			return BatchResult{}, invalidArgument(operation, "Ad Group name, regions, negative keywords, or tracking parameters are invalid")
		}
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return BatchResult{}, err
	}
	ctx, cancel := withCallTimeout(ctx, callOptions.Timeout)
	defer cancel()
	options = nil
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return BatchResult{}, withOperation(err, operation)
	}
	if campaign.Type != CampaignText || !safeNonServingCampaign(campaign) {
		return BatchResult{}, invalidArgument(operation, "parent must be a classic Text Campaign that is suspended or draft/off")
	}
	type write struct {
		AdGroupAdd
		CampaignID int64 `json:"CampaignId"`
	}
	items := make([]write, len(inputs))
	for index, input := range inputs {
		items[index] = write{AdGroupAdd: input, CampaignID: campaignID}
	}
	params := struct {
		AdGroups []write `json:"AdGroups"`
	}{AdGroups: items}
	var response struct {
		AddResults []ActionResult `json:"AddResults"`
	}
	metadata, err := client.rpc(ctx, operation, "adgroups", "add", params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	return actionResult(operation, response.AddResults, len(inputs), metadata)
}

func (client *Client) UpdateAdGroups(ctx context.Context, inputs []AdGroupUpdate, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "adgroup_update"
	if len(inputs) == 0 || len(inputs) > MaximumAdGroupMutationBatch {
		return BatchResult{}, invalidArgument(operation, "1-1000 Ad Group updates are required")
	}
	seen := make(map[int64]struct{}, len(inputs))
	ids := make([]int64, len(inputs))
	for index, input := range inputs {
		if !validAdGroupUpdate(input) {
			return BatchResult{}, invalidArgument(operation, "Ad Group update is invalid")
		}
		if _, exists := seen[input.ID]; exists {
			return BatchResult{}, invalidArgument(operation, "Ad Group IDs must be unique")
		}
		seen[input.ID] = struct{}{}
		ids[index] = input.ID
	}
	params := struct {
		AdGroups []AdGroupUpdate `json:"AdGroups"`
	}{AdGroups: inputs}
	var response struct {
		UpdateResults []ActionResult `json:"UpdateResults"`
	}
	metadata, err := client.rpc(ctx, operation, "adgroups", "update", params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	return actionResult(operation, response.UpdateResults, len(inputs), metadata, ids...)
}

func (client *Client) DeleteAdGroups(ctx context.Context, ids []int64, options ...socialhub.CallOption) (BatchResult, error) {
	const operation = "adgroup_delete"
	if !validIDs(ids, MaximumAdGroupMutationBatch, false) {
		return BatchResult{}, invalidArgument(operation, "1-1000 unique Ad Group IDs are required")
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return BatchResult{}, err
	}
	ctx, cancel := withCallTimeout(ctx, callOptions.Timeout)
	defer cancel()
	options = nil
	page, err := client.ListAdGroups(ctx, ListAdGroupsRequest{
		Selection: AdGroupSelection{IDs: ids}, Page: PageRequest{Limit: int64(len(ids))},
	}, options...)
	if err != nil {
		return BatchResult{}, withOperation(err, operation)
	}
	if len(page.Items) != len(ids) {
		return BatchResult{}, notFound(operation, "one or more Ad Groups were not returned")
	}
	expected := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		expected[id] = struct{}{}
	}
	campaigns := make(map[int64]struct{})
	for _, group := range page.Items {
		if _, found := expected[group.ID]; !found {
			return BatchResult{}, platformContractError(operation, "Yandex returned a duplicate Ad Group or one outside the delete selection")
		}
		delete(expected, group.ID)
		campaigns[group.CampaignID] = struct{}{}
	}
	if len(expected) != 0 {
		return BatchResult{}, platformContractError(operation, "Yandex omitted one or more Ad Groups from the delete selection")
	}
	campaignIDs := make([]int64, 0, len(campaigns))
	for campaignID := range campaigns {
		campaignIDs = append(campaignIDs, campaignID)
	}
	sort.Slice(campaignIDs, func(i, j int) bool { return campaignIDs[i] < campaignIDs[j] })
	for _, campaignID := range campaignIDs {
		campaign, err := client.GetCampaign(ctx, campaignID, options...)
		if err != nil {
			return BatchResult{}, withOperation(err, operation)
		}
		if !safeNonServingCampaign(campaign) {
			return BatchResult{}, invalidArgument(operation, "every parent Campaign must be suspended or draft/off before Ad Group delete")
		}
	}
	params := struct {
		SelectionCriteria struct {
			IDs []int64 `json:"Ids"`
		} `json:"SelectionCriteria"`
	}{}
	params.SelectionCriteria.IDs = ids
	var response struct {
		DeleteResults []ActionResult `json:"DeleteResults"`
	}
	metadata, err := client.rpc(ctx, operation, "adgroups", "delete", params, &response, true, options...)
	if err != nil {
		return BatchResult{Metadata: metadata}, err
	}
	return actionResult(operation, response.DeleteResults, len(ids), metadata, ids...)
}

func validateAdGroup(operation string, group *AdGroup, expectedID int64) error {
	if group == nil || group.ID <= 0 || group.CampaignID <= 0 || !validText(group.Name, 255) ||
		!validRegions(group.RegionIDs) || group.Status == "" || group.Type == "" {
		return platformContractError(operation, "Yandex returned an invalid Ad Group")
	}
	if expectedID > 0 && group.ID != expectedID {
		return platformContractError(operation, "Ad Group ID did not match the request")
	}
	if !validStringArray(group.NegativeKeywords, 1000, 4096, true) ||
		!validInt64Array(group.NegativeKeywordSharedSetIDs, 3, true) {
		return platformContractError(operation, "Yandex returned invalid Ad Group targeting")
	}
	return nil
}
