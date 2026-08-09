package ads

import (
	"context"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

type createAdGroupData struct {
	CampaignID        string           `json:"campaign_id"`
	ConfiguredStatus  ConfiguredStatus `json:"configured_status"`
	Name              string           `json:"name"`
	BidType           BidType          `json:"bid_type"`
	StartTime         string           `json:"start_time"`
	EndTime           string           `json:"end_time,omitempty"`
	BidStrategy       *BidStrategy     `json:"bid_strategy"`
	BidValue          *int64           `json:"bid_value"`
	GoalType          GoalType         `json:"goal_type,omitempty"`
	GoalValue         *int64           `json:"goal_value,omitempty"`
	ConversionPixelID string           `json:"conversion_pixel_id"`
}

func (client *Client) ListAdGroups(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[AdGroup], error) {
	const operation = "ad_groups_list"
	if !validList(input) {
		return socialhub.Page[AdGroup]{}, invalidArgument(operation, "pagination is invalid")
	}
	path := client.accountPath("ad_groups")
	var response listResponse[AdGroup]
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return socialhub.Page[AdGroup]{}, err
	}
	for index := range response.Data {
		if err := client.validateAdGroup(operation, &response.Data[index], "", ""); err != nil {
			return socialhub.Page[AdGroup]{}, err
		}
	}
	cursor, err := client.pageCursor(operation, path, response.Pagination.NextURL)
	if err != nil {
		return socialhub.Page[AdGroup]{}, err
	}
	return page(response.Data, cursor), nil
}

func (client *Client) GetAdGroup(ctx context.Context, id string, options ...socialhub.CallOption) (*AdGroup, error) {
	return client.getAdGroup(ctx, "ad_group_get", id, options...)
}

func (client *Client) getAdGroup(ctx context.Context, operation, id string, options ...socialhub.CallOption) (*AdGroup, error) {
	if !validResourceID(id) {
		return nil, invalidArgument(operation, "Ad Group ID must be numeric")
	}
	var response singleResponse[AdGroup]
	if _, err := client.getJSON(ctx, operation, "/ad_groups/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &response.Data, id, ""); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) CreateAdGroup(ctx context.Context, input CreateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_create"
	if !validResourceID(input.CampaignID) || !validText(input.Name, 500) || !validBidType(input.BidType) ||
		input.StartTime.IsZero() || input.EndTime != nil && (input.EndTime.IsZero() || !input.EndTime.After(input.StartTime)) ||
		!validPixelID(input.ConversionPixelID) || input.BidValue != nil && *input.BidValue <= 0 || input.GoalValue != nil && *input.GoalValue <= 0 {
		return nil, invalidArgument(operation, "Campaign, name, bid, schedule, goal, or conversion_pixel_id is invalid")
	}
	campaign, err := client.getCampaign(ctx, operation, input.CampaignID, options...)
	if err != nil {
		return nil, err
	}
	if campaignCBO(*campaign) {
		if input.BidStrategy != nil || input.BidValue != nil || input.GoalType != "" || input.GoalValue != nil {
			return nil, invalidArgument(operation, "CBO Ad Groups must inherit bid and goal settings from the Campaign")
		}
	} else if input.BidStrategy == nil || !validBidStrategy(*input.BidStrategy) || !validGoalType(input.GoalType) || input.GoalValue == nil || *input.GoalValue <= 0 {
		return nil, invalidArgument(operation, "non-CBO Ad Groups require bid strategy, goal type, and positive goal value")
	}
	if input.BidStrategy != nil && (*input.BidStrategy == BidStrategyManual || *input.BidStrategy == BidStrategyTargetCPX) && input.BidValue == nil {
		return nil, invalidArgument(operation, "manual and target Ad Group bidding require a positive bid value")
	}
	data := createAdGroupData{
		CampaignID: input.CampaignID, ConfiguredStatus: StatusPaused, Name: input.Name,
		BidType: input.BidType, StartTime: input.StartTime.UTC().Format(time.RFC3339),
		BidStrategy: input.BidStrategy, BidValue: input.BidValue, GoalType: input.GoalType,
		GoalValue: input.GoalValue, ConversionPixelID: input.ConversionPixelID,
	}
	if input.EndTime != nil {
		data.EndTime = input.EndTime.UTC().Format(time.RFC3339)
	}
	var response singleResponse[AdGroup]
	path := client.accountPath("ad_groups")
	if _, err := client.writeJSON(ctx, operation, http.MethodPost, path, nil, struct {
		Data createAdGroupData `json:"data"`
	}{Data: data}, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &response.Data, "", input.CampaignID); err != nil {
		return nil, err
	}
	if response.Data.ConfiguredStatus != StatusPaused || response.Data.ConversionPixelID != input.ConversionPixelID {
		return nil, platformContractError(operation, "Reddit did not preserve the paused Ad Group safety settings")
	}
	return &response.Data, nil
}

func (client *Client) UpdateAdGroup(ctx context.Context, id string, input UpdateAdGroupRequest, options ...socialhub.CallOption) (*AdGroup, error) {
	const operation = "ad_group_update"
	if !validResourceID(id) || input.Name == nil && input.Status == nil ||
		input.Name != nil && !validText(*input.Name, 500) || input.Status != nil && !validMutationStatus(*input.Status) {
		return nil, invalidArgument(operation, "Ad Group ID or update fields are invalid")
	}
	current, err := client.getAdGroup(ctx, operation, id, options...)
	if err != nil {
		return nil, err
	}
	if _, err := client.getCampaign(ctx, operation, current.CampaignID, options...); err != nil {
		return nil, err
	}
	data := make(map[string]any, 2)
	if input.Name != nil {
		data["name"] = *input.Name
	}
	if input.Status != nil {
		data["configured_status"] = *input.Status
	}
	var response singleResponse[AdGroup]
	if _, err := client.writeJSON(ctx, operation, http.MethodPatch, "/ad_groups/"+id, nil, struct {
		Data map[string]any `json:"data"`
	}{Data: data}, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateAdGroup(operation, &response.Data, id, current.CampaignID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) validateAdGroup(operation string, value *AdGroup, expectedID, expectedCampaignID string) error {
	if !validResourceID(value.ID) || expectedID != "" && value.ID != expectedID || !validResourceID(value.CampaignID) ||
		expectedCampaignID != "" && value.CampaignID != expectedCampaignID {
		return platformContractError(operation, "Reddit returned a missing or mismatched Ad Group identity")
	}
	if value.AdAccountID != "" && value.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Reddit returned an Ad Group owned by another Ad Account")
	}
	if value.AdAccountID == "" {
		value.AdAccountID = client.adAccountID
	}
	return nil
}
