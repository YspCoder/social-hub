package naversearchads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type adGroupWrite struct {
	ID                       string          `json:"nccAdgroupId,omitempty"`
	CampaignID               string          `json:"nccCampaignId,omitempty"`
	CustomerID               int64           `json:"customerId,omitempty"`
	Name                     string          `json:"name,omitempty"`
	Type                     AdGroupType     `json:"adgroupType,omitempty"`
	PCChannelID              string          `json:"pcChannelId,omitempty"`
	MobileChannelID          string          `json:"mobileChannelId,omitempty"`
	BidAmount                *int64          `json:"bidAmt,omitempty"`
	DailyBudget              *int64          `json:"dailyBudget,omitempty"`
	UseDailyBudget           *bool           `json:"useDailyBudget,omitempty"`
	ContentsNetworkBidAmount *int64          `json:"contentsNetworkBidAmt,omitempty"`
	UseContentsNetworkBid    *bool           `json:"useCntsNetworkBidAmt,omitempty"`
	AdRollingType            AdRollingType   `json:"adRollingType,omitempty"`
	Attributes               json.RawMessage `json:"adgroupAttrJson,omitempty"`
	AutoBidStrategy          json.RawMessage `json:"autobidStrategy,omitempty"`
	Targets                  json.RawMessage `json:"targets,omitempty"`
	UserLock                 *bool           `json:"userLock,omitempty"`
}

func (client *Client) ListAdGroups(ctx context.Context, input ListAdGroupsRequest, options ...socialhub.CallOption) (Page[AdGroup], error) {
	const operation = "adgroup_list"
	if !validID(input.CampaignID) || !validListOptions(input.ListOptions) {
		return Page[AdGroup]{}, invalidArgument(operation, "campaign ID or list options are invalid")
	}
	list := normalizeList(input.ListOptions)
	query := listValues(list)
	query.Set("nccCampaignId", input.CampaignID)
	var groups []AdGroup
	if err := client.getJSON(ctx, operation, "/ncc/adgroups", query, &groups, options...); err != nil {
		return Page[AdGroup]{}, err
	}
	for index := range groups {
		if err := client.validateAdGroup(operation, &groups[index], "", input.CampaignID); err != nil {
			return Page[AdGroup]{}, err
		}
	}
	return listPage(groups, list, func(value AdGroup) string { return value.ID }), nil
}

func (client *Client) GetAdGroup(ctx context.Context, id string, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_get"
	if !validID(id) {
		return nil, invalidArgument(operation, "Ad Group ID is invalid")
	}
	var group AdGroup
	if err := client.getJSON(ctx, operation, "/ncc/adgroups/"+id, nil, &group, options...); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &group, id, ""); err != nil {
		return nil, err
	}
	return &group, nil
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_create"
	if !validCreateAdGroup(input) {
		return nil, invalidArgument(operation, "Ad Group campaign, name, channels, bids, budget, or extensions are invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	campaign, err := client.GetCampaign(ctx, input.CampaignID, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if !campaign.UserLock || campaign.Status != StatusPaused {
		return nil, invalidArgument(operation, "parent Campaign must be paused before creating an Ad Group")
	}
	paused := true
	payload := adGroupWrite{
		CampaignID: input.CampaignID, Name: input.Name, Type: input.Type,
		PCChannelID: input.PCChannelID, MobileChannelID: input.MobileChannelID,
		BidAmount: input.BidAmount, DailyBudget: input.DailyBudget, UseDailyBudget: input.UseDailyBudget,
		ContentsNetworkBidAmount: input.ContentsNetworkBidAmount, UseContentsNetworkBid: input.UseContentsNetworkBid,
		AdRollingType: input.AdRollingType, Attributes: cloneRaw(input.Attributes),
		AutoBidStrategy: cloneRaw(input.AutoBidStrategy), Targets: cloneRaw(input.Targets), UserLock: &paused,
	}
	var group AdGroup
	if err := client.writeJSON(ctx, operation, http.MethodPost, "/ncc/adgroups", nil, payload, &group, prepared...); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &group, "", input.CampaignID); err != nil {
		return nil, outcomeUnknownError(operation, err)
	}
	if !group.UserLock || group.Status != StatusPaused {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "created Ad Group was not paused"))
	}
	return &group, nil
}

func (client *Client) UpdateAdGroupBid(ctx context.Context, id string, bidAmount int64, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_bid"
	if !validID(id) || bidAmount < 70 || bidAmount > 100_000 {
		return nil, invalidArgument(operation, "Ad Group ID or bid amount is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	current, err := client.GetAdGroup(ctx, id, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	payload := adGroupPayload(current)
	payload.BidAmount = &bidAmount
	return client.updateAdGroup(ctx, operation, id, "bidAmt", payload, prepared...)
}

func (client *Client) UpdateAdGroupBudget(ctx context.Context, id string, input AdGroupBudgetUpdate, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_budget"
	if !validID(id) || input.DailyBudget < 0 || input.UseDailyBudget && input.DailyBudget < 70 || !input.UseDailyBudget && input.DailyBudget != 0 {
		return nil, invalidArgument(operation, "Ad Group ID or budget is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	current, err := client.GetAdGroup(ctx, id, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	payload := adGroupPayload(current)
	payload.DailyBudget = &input.DailyBudget
	payload.UseDailyBudget = &input.UseDailyBudget
	return client.updateAdGroup(ctx, operation, id, "budget", payload, prepared...)
}

func (client *Client) SetAdGroupPaused(ctx context.Context, id string, paused bool, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_set_paused"
	if !validID(id) {
		return nil, invalidArgument(operation, "Ad Group ID is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	current, err := client.GetAdGroup(ctx, id, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if current.Status == StatusDeleted {
		return nil, invalidArgument(operation, "deleted Ad Group cannot be changed")
	}
	if !paused {
		campaign, err := client.GetCampaign(ctx, current.CampaignID, prepared...)
		if err != nil {
			return nil, withOperation(err, operation)
		}
		if campaign.UserLock || campaign.Status != StatusEligible {
			return nil, invalidArgument(operation, "parent Campaign must be eligible and enabled before enabling an Ad Group")
		}
	}
	payload := adGroupPayload(current)
	payload.UserLock = &paused
	group, err := client.updateAdGroup(ctx, operation, id, "userLock", payload, prepared...)
	if err != nil {
		return nil, err
	}
	if group.UserLock != paused {
		return nil, outcomeUnknownError(operation, platformContractError(operation, "Ad Group lock did not match the requested state"))
	}
	return group, nil
}

func (client *Client) DeleteAdGroup(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "adgroup_delete"
	if !validID(id) {
		return invalidArgument(operation, "Ad Group ID is invalid")
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	current, err := client.GetAdGroup(ctx, id, prepared...)
	if err != nil {
		return withOperation(err, operation)
	}
	if !current.UserLock || current.Status != StatusPaused {
		return invalidArgument(operation, "Ad Group must be paused before deletion")
	}
	return client.delete(ctx, operation, "/ncc/adgroups/"+id, prepared...)
}

func (client *Client) updateAdGroup(ctx context.Context, operation, id, field string, payload adGroupWrite, options ...socialhub.CallOption) (*AdGroup, error) {
	query := url.Values{"fields": {field}}
	var group AdGroup
	if err := client.writeJSON(ctx, operation, http.MethodPut, "/ncc/adgroups/"+id, query, payload, &group, options...); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &group, id, payload.CampaignID); err != nil {
		return nil, outcomeUnknownError(operation, err)
	}
	return &group, nil
}

func (client *Client) validateAdGroup(operation string, group *AdGroup, expectedID, expectedCampaignID string) error {
	if group == nil || !validID(group.ID) || !validID(group.CampaignID) || group.CustomerID != client.customerID {
		return platformContractError(operation, "Ad Group response has invalid IDs or customer ownership")
	}
	if expectedID != "" && group.ID != expectedID || expectedCampaignID != "" && group.CampaignID != expectedCampaignID {
		return platformContractError(operation, "Ad Group response ownership did not match the request")
	}
	return nil
}

func adGroupPayload(value *AdGroup) adGroupWrite {
	return adGroupWrite{
		ID: value.ID, CampaignID: value.CampaignID, CustomerID: value.CustomerID,
		Name: value.Name, Type: value.Type, PCChannelID: value.PCChannelID, MobileChannelID: value.MobileChannelID,
		BidAmount: &value.BidAmount, DailyBudget: &value.DailyBudget, UseDailyBudget: &value.UseDailyBudget,
		ContentsNetworkBidAmount: &value.ContentsNetworkBidAmount, UseContentsNetworkBid: &value.UseContentsNetworkBid,
		AdRollingType: value.AdRollingType, Attributes: cloneRaw(value.Attributes),
		AutoBidStrategy: cloneRaw(value.AutoBidStrategy), Targets: cloneRaw(value.Targets), UserLock: &value.UserLock,
	}
}

func validCreateAdGroup(input CreateAdGroupRequest) bool {
	if !validID(input.CampaignID) || !validText(input.Name, 128) || !validAdGroupType(input.Type, true) ||
		input.PCChannelID == "" && input.MobileChannelID == "" || !validOpaqueOptional(input.PCChannelID, 128) ||
		!validOpaqueOptional(input.MobileChannelID, 128) || !validRawObject(input.Attributes, true) ||
		!validRawObject(input.AutoBidStrategy, true) || !validRawArray(input.Targets, true) {
		return false
	}
	if input.BidAmount != nil && (*input.BidAmount < 70 || *input.BidAmount > 100_000) ||
		input.ContentsNetworkBidAmount != nil && (*input.ContentsNetworkBidAmount < 70 || *input.ContentsNetworkBidAmount > 100_000) {
		return false
	}
	if input.DailyBudget != nil && (*input.DailyBudget < 0 || *input.DailyBudget > 1_000_000_000) {
		return false
	}
	if input.UseDailyBudget != nil && (*input.UseDailyBudget && (input.DailyBudget == nil || *input.DailyBudget < 70) ||
		!*input.UseDailyBudget && input.DailyBudget != nil && *input.DailyBudget != 0) {
		return false
	}
	return input.AdRollingType == "" || input.AdRollingType == AdRollingRoundRobin || input.AdRollingType == AdRollingPerformance
}
