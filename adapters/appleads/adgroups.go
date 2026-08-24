package appleads

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAdGroups(ctx context.Context, campaignID int64, pagination Pagination, options ...socialhub.CallOption) (Page[AdGroup], error) {
	const operation = "adgroups_list"
	if !validID(campaignID) || !validPagination(pagination) {
		return Page[AdGroup]{}, invalidArgument(operation, "campaign ID or pagination is invalid")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return Page[AdGroup]{}, err
	}
	return client.listAdGroups(ctx, operation, campaignID, pagination, options...)
}

func (client *Client) listAdGroups(ctx context.Context, operation string, campaignID int64, pagination Pagination, options ...socialhub.CallOption) (Page[AdGroup], error) {
	var response responseEnvelope[[]AdGroup]
	if err := client.getJSON(ctx, operation, "/campaigns/"+formatID(campaignID)+"/adgroups", listQuery(pagination), &response, options...); err != nil {
		return Page[AdGroup]{}, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return Page[AdGroup]{}, err
	}
	for index := range response.Data {
		if err := client.validateAdGroup(operation, &response.Data[index], campaignID, 0); err != nil {
			return Page[AdGroup]{}, err
		}
	}
	return pageResult(response.Data, response.Pagination), nil
}

func (client *Client) GetAdGroup(ctx context.Context, campaignID, adGroupID int64, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_get"
	if !validID(campaignID) || !validID(adGroupID) {
		return nil, invalidArgument(operation, "campaign ID and Ad Group ID must be positive")
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	return client.getAdGroup(ctx, operation, campaignID, adGroupID, options...)
}

func (client *Client) getAdGroup(ctx context.Context, operation string, campaignID, adGroupID int64, options ...socialhub.CallOption) (*AdGroup, error) {
	var response responseEnvelope[AdGroup]
	path := "/campaigns/" + formatID(campaignID) + "/adgroups/" + formatID(adGroupID)
	if err := client.getJSON(ctx, operation, path, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &response.Data, campaignID, adGroupID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

type adGroupWrite struct {
	CampaignID                int64           `json:"campaignId,omitempty"`
	OrgID                     int64           `json:"orgId,omitempty"`
	Name                      *string         `json:"name,omitempty"`
	CPAGoal                   *Money          `json:"cpaGoal,omitempty"`
	DefaultBidAmount          *Money          `json:"defaultBidAmount,omitempty"`
	PricingModel              string          `json:"pricingModel,omitempty"`
	AutomatedKeywordsOptIn    *bool           `json:"automatedKeywordsOptIn,omitempty"`
	AutomatedKeywordsRequired bool            `json:"automatedKeywordsRequired,omitempty"`
	TargetingDimensions       json.RawMessage `json:"targetingDimensions,omitempty"`
	StartTime                 *string         `json:"startTime,omitempty"`
	EndTime                   *string         `json:"endTime,omitempty"`
	Status                    *AdGroupStatus  `json:"status,omitempty"`
}

func (client *Client) CreateAdGroup(ctx context.Context, campaignID int64, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_create"
	if !validID(campaignID) || !validCreateAdGroup(input) {
		return nil, invalidArgument(operation, "campaign ID or Ad Group fields are invalid")
	}
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return nil, err
	}
	if campaign.Status != CampaignPaused {
		return nil, invalidArgument(operation, "Campaign must be paused before creating an Ad Group")
	}
	paused := AdGroupPaused
	payload := adGroupWrite{
		CampaignID: campaignID, OrgID: client.orgID, Name: &input.Name, CPAGoal: input.CPAGoal,
		DefaultBidAmount: input.DefaultBidAmount, PricingModel: input.PricingModel,
		AutomatedKeywordsOptIn: &input.AutomatedKeywordsOptIn, AutomatedKeywordsRequired: input.AutomatedKeywordsRequired,
		TargetingDimensions: append(json.RawMessage(nil), input.TargetingDimensions...), Status: &paused,
	}
	if input.StartTime != "" {
		payload.StartTime = &input.StartTime
	}
	if input.EndTime != "" {
		payload.EndTime = &input.EndTime
	}
	var response responseEnvelope[AdGroup]
	path := "/campaigns/" + formatID(campaignID) + "/adgroups"
	if err := client.postJSON(ctx, operation, path, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &response.Data, campaignID, 0); err != nil {
		return nil, err
	}
	if response.Data.Status != AdGroupPaused {
		return nil, platformContractError(operation, "created Ad Group was not paused")
	}
	return &response.Data, nil
}

func (client *Client) UpdateAdGroup(ctx context.Context, campaignID, adGroupID int64, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_update"
	if !validID(campaignID) || !validID(adGroupID) || !validUpdateAdGroup(input) {
		return nil, invalidArgument(operation, "campaign ID, Ad Group ID, or update fields are invalid")
	}
	if _, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	payload := adGroupWrite{
		Name: input.Name, CPAGoal: input.CPAGoal, DefaultBidAmount: input.DefaultBidAmount,
		AutomatedKeywordsOptIn: input.AutomatedKeywordsOptIn,
		TargetingDimensions:    append(json.RawMessage(nil), input.TargetingDimensions...),
		StartTime:              input.StartTime, EndTime: input.EndTime,
	}
	return client.writeAdGroup(ctx, operation, campaignID, adGroupID, payload, nil, options...)
}

func (client *Client) SetAdGroupEnabled(ctx context.Context, campaignID, adGroupID int64, enabled bool, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "adgroup_set_enabled"
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.getAdGroup(ctx, operation, campaignID, adGroupID, options...); err != nil {
		return nil, err
	}
	if enabled && campaign.Status != CampaignEnabled {
		return nil, invalidArgument(operation, "Campaign must be enabled before enabling an Ad Group")
	}
	status := AdGroupPaused
	if enabled {
		status = AdGroupEnabled
	}
	return client.writeAdGroup(ctx, operation, campaignID, adGroupID, adGroupWrite{Status: &status}, &status, options...)
}

func (client *Client) DeleteAdGroup(ctx context.Context, campaignID, adGroupID int64, options ...socialhub.CallOption) error {
	const operation = "adgroup_delete"
	current, err := client.GetAdGroup(ctx, campaignID, adGroupID, options...)
	if err != nil {
		return err
	}
	if current.Status != AdGroupPaused {
		return invalidArgument(operation, "Ad Group must be paused before deletion")
	}
	var response responseEnvelope[json.RawMessage]
	path := "/campaigns/" + formatID(campaignID) + "/adgroups/" + formatID(adGroupID)
	if err := client.deleteJSON(ctx, operation, path, &response, options...); err != nil {
		return err
	}
	return checkEnvelopeError(operation, response.Error)
}

func (client *Client) writeAdGroup(ctx context.Context, operation string, campaignID, adGroupID int64, payload adGroupWrite, expected *AdGroupStatus, options ...socialhub.CallOption) (*AdGroup, error) {
	var response responseEnvelope[AdGroup]
	path := "/campaigns/" + formatID(campaignID) + "/adgroups/" + formatID(adGroupID)
	if err := client.putJSON(ctx, operation, path, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &response.Data, campaignID, adGroupID); err != nil {
		return nil, err
	}
	if expected != nil && response.Data.Status != *expected {
		return nil, platformContractError(operation, "Ad Group status did not match the requested state")
	}
	return &response.Data, nil
}

func (client *Client) validateAdGroup(operation string, group *AdGroup, campaignID, expectedID int64) error {
	if group == nil || !validID(group.ID) || group.OrgID != client.orgID || group.CampaignID != campaignID {
		return platformContractError(operation, "Ad Group response has invalid ID or parent ownership")
	}
	if expectedID != 0 && group.ID != expectedID {
		return platformContractError(operation, "Ad Group response ID did not match the requested Ad Group")
	}
	return nil
}

func validCreateAdGroup(input CreateAdGroupRequest) bool {
	if !validText(input.Name, 200) || input.PricingModel != "CPC" || !validPositiveMoney(input.CPAGoal) ||
		!validPositiveMoney(input.DefaultBidAmount) || !validRawObject(input.TargetingDimensions) ||
		!validDateTime(input.StartTime) || !validDateTime(input.EndTime) || input.StartTime != "" && input.EndTime != "" && input.EndTime <= input.StartTime {
		return false
	}
	if input.AutomatedKeywordsRequired && !input.AutomatedKeywordsOptIn {
		return false
	}
	return input.AutomatedKeywordsRequired || input.DefaultBidAmount != nil
}

func validUpdateAdGroup(input UpdateAdGroupRequest) bool {
	if input.Name == nil && input.CPAGoal == nil && input.DefaultBidAmount == nil && input.AutomatedKeywordsOptIn == nil &&
		len(input.TargetingDimensions) == 0 && input.StartTime == nil && input.EndTime == nil {
		return false
	}
	return validOptionalText(input.Name, 200) && validPositiveMoney(input.CPAGoal) && validPositiveMoney(input.DefaultBidAmount) &&
		validRawObject(input.TargetingDimensions) && (input.StartTime == nil || validDateTime(*input.StartTime)) &&
		(input.EndTime == nil || validDateTime(*input.EndTime))
}
