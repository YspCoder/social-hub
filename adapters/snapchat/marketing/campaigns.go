package marketing

import (
	"context"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

type campaignItem struct {
	SubRequestStatus      string     `json:"sub_request_status"`
	SubRequestErrorReason string     `json:"sub_request_error_reason,omitempty"`
	Errors                []apiError `json:"errors,omitempty"`
	Campaign              *Campaign  `json:"campaign"`
}

type campaignResponse struct {
	responseMeta
	Campaigns []campaignItem `json:"campaigns"`
	Paging    paging         `json:"paging"`
}

type createCampaignPayload struct {
	Name                  string                `json:"name"`
	AdAccountID           string                `json:"ad_account_id"`
	Status                EntityStatus          `json:"status"`
	StartTime             string                `json:"start_time"`
	BuyModel              string                `json:"buy_model"`
	CreationState         string                `json:"creation_state"`
	ObjectiveV2Properties ObjectiveV2Properties `json:"objective_v2_properties"`
}

func (client *Client) ListCampaigns(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[Campaign], error) {
	const operation = "campaigns_list"
	if !validPage(input.Cursor, input.Limit, 1000) {
		return socialhub.Page[Campaign]{}, invalidArgument(operation, "cursor or limit is invalid")
	}
	path := client.accountResourcePath("campaigns")
	var response campaignResponse
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	items, err := client.campaignItems(operation, response)
	if err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	cursor, err := client.pageCursor(operation, path, response.Paging.NextLink)
	if err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	return socialhub.Page[Campaign]{Items: items, NextCursor: cursor, HasMore: cursor != nil}, nil
}

func (client *Client) GetCampaign(ctx context.Context, id string, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_get"
	if !validUUID(id) {
		return nil, invalidArgument(operation, "Campaign ID must be a UUID")
	}
	var response campaignResponse
	if _, err := client.getJSON(ctx, operation, "/campaigns/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	return client.singleCampaign(operation, response, id)
}

func (client *Client) CreateCampaign(ctx context.Context, input CreateCampaignRequest, options ...socialhub.CallOption) (*Campaign, error) {
	const operation = "campaign_create"
	if !validText(input.Name, 256) || input.Objective != ObjectiveAwarenessAndEngagement || input.StartTime.IsZero() {
		return nil, invalidArgument(operation, "name, supported objective, and start time are required")
	}
	payload := struct {
		Campaigns []createCampaignPayload `json:"campaigns"`
	}{Campaigns: []createCampaignPayload{{
		Name: input.Name, AdAccountID: client.adAccountID, Status: StatusPaused,
		StartTime: input.StartTime.UTC().Format(time.RFC3339), BuyModel: "AUCTION", CreationState: "PUBLISHED",
		ObjectiveV2Properties: ObjectiveV2Properties{Type: input.Objective},
	}}}
	var response campaignResponse
	if _, err := client.writeJSON(ctx, operation, http.MethodPost, client.accountResourcePath("campaigns"), payload, &response, options...); err != nil {
		return nil, err
	}
	created, err := client.singleCampaign(operation, response, "")
	if err != nil {
		return nil, err
	}
	return client.GetCampaign(ctx, created.ID, options...)
}

func (client *Client) UpdateCampaign(ctx context.Context, id string, input UpdateEntityRequest, options ...socialhub.CallOption) (*Campaign, error) {
	return client.patchCampaign(ctx, "campaign_update", id, input, options...)
}

func (client *Client) SetCampaignStatus(ctx context.Context, id string, status EntityStatus, options ...socialhub.CallOption) (*Campaign, error) {
	return client.patchCampaign(ctx, "campaign_status", id, UpdateEntityRequest{Status: &status}, options...)
}

func (client *Client) patchCampaign(ctx context.Context, operation, id string, input UpdateEntityRequest, options ...socialhub.CallOption) (*Campaign, error) {
	operations, err := updateOperations(operation, id, input)
	if err != nil {
		return nil, err
	}
	if _, err := client.GetCampaign(ctx, id, options...); err != nil {
		return nil, err
	}
	var response campaignResponse
	path := client.accountResourcePath("campaigns/" + id)
	if _, err := client.writeJSON(ctx, operation, http.MethodPatch, path, operations, &response, options...); err != nil {
		return nil, err
	}
	if _, err := client.singleCampaign(operation, response, id); err != nil {
		return nil, err
	}
	return client.GetCampaign(ctx, id, options...)
}

func (client *Client) campaignItems(operation string, response campaignResponse) ([]Campaign, error) {
	states := make([]subRequestState, len(response.Campaigns))
	for index, item := range response.Campaigns {
		states[index] = subRequestState{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason, Errors: item.Errors}
	}
	if err := checkResponse(operation, response.responseMeta, states); err != nil {
		return nil, err
	}
	items := make([]Campaign, len(response.Campaigns))
	for index, item := range response.Campaigns {
		if item.Campaign == nil {
			return nil, platformContractError(operation, "Snapchat Campaign result omitted the Campaign")
		}
		if err := client.validateCampaign(operation, item.Campaign, ""); err != nil {
			return nil, err
		}
		items[index] = *item.Campaign
	}
	return items, nil
}

func (client *Client) singleCampaign(operation string, response campaignResponse, expectedID string) (*Campaign, error) {
	if len(response.Campaigns) != 1 {
		return nil, platformContractError(operation, "Snapchat did not return exactly one Campaign result")
	}
	items, err := client.campaignItems(operation, response)
	if err != nil {
		return nil, err
	}
	if expectedID != "" && items[0].ID != expectedID {
		return nil, platformContractError(operation, "Snapchat returned a mismatched Campaign ID")
	}
	return &items[0], nil
}

func (client *Client) validateCampaign(operation string, campaign *Campaign, expectedID string) error {
	if !validUUID(campaign.ID) || expectedID != "" && campaign.ID != expectedID {
		return platformContractError(operation, "Snapchat returned a missing or mismatched Campaign ID")
	}
	if campaign.AdAccountID != client.adAccountID {
		return platformContractError(operation, "Snapchat returned a Campaign owned by another Ad Account")
	}
	return nil
}

func updateOperations(operation, id string, input UpdateEntityRequest) ([]jsonPatchOperation, error) {
	if !validUUID(id) || input.Name == nil && input.Status == nil {
		return nil, invalidArgument(operation, "resource ID and at least one update are required")
	}
	operations := make([]jsonPatchOperation, 0, 2)
	if input.Name != nil {
		if !validText(*input.Name, 256) {
			return nil, invalidArgument(operation, "name is invalid")
		}
		operations = append(operations, jsonPatchOperation{Op: "replace", Path: "/name", Value: *input.Name})
	}
	if input.Status != nil {
		if !validStatus(*input.Status) {
			return nil, invalidArgument(operation, "status must be ACTIVE or PAUSED")
		}
		operations = append(operations, jsonPatchOperation{Op: "replace", Path: "/status", Value: *input.Status})
	}
	return operations, nil
}
