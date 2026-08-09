package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCreatives(ctx context.Context, input ListCreativesRequest, options ...socialhub.CallOption) (NumberPage[Creative], error) {
	const operation = "creative_list"
	if !validateIDs(input.IDs, 100) || input.CampaignID < 0 || input.UnitID < 0 || !validateStatuses(input.PutStatuses) ||
		!validateDatePair(input.StartDate, input.EndDate) || input.Name != "" && !validRequiredText(input.Name, 100) ||
		input.TimeFilterType < 0 || input.TimeFilterType > 1 {
		return NumberPage[Creative]{}, invalidArgument(operation, "filters, dates, statuses, or IDs are invalid")
	}
	page, pageSize, err := validatePage(input.Page, input.PageSize, 200)
	if err != nil {
		return NumberPage[Creative]{}, err
	}
	body := map[string]any{"advertiser_id": client.advertiserID, "page": page, "page_size": pageSize}
	if len(input.IDs) > 0 {
		body["creative_ids"] = input.IDs
	}
	if input.CampaignID > 0 {
		body["campaign_id"] = input.CampaignID
	}
	if input.UnitID > 0 {
		body["unit_id"] = input.UnitID
	}
	if input.Name != "" {
		body["creative_name"] = input.Name
	}
	if len(input.PutStatuses) > 0 {
		body["put_status_list"] = input.PutStatuses
	}
	if input.StartDate != "" {
		body["start_date"], body["end_date"] = input.StartDate, input.EndDate
		body["time_filter_type"] = input.TimeFilterType
	}
	var response apiEnvelope[struct {
		Details    []Creative `json:"details"`
		TotalCount int64      `json:"total_count"`
	}]
	header, err := client.postJSON(ctx, operation, "/gw/dsp/creative/list", body, &response, options...)
	if err != nil {
		return NumberPage[Creative]{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return NumberPage[Creative]{}, err
	}
	for index := range data.Details {
		if err := requireResourceID(operation, 0, data.Details[index].ID); err != nil {
			return NumberPage[Creative]{}, err
		}
		if err := requireAdvertiser(operation, client.advertiserID, data.Details[index].AdvertiserID); err != nil {
			return NumberPage[Creative]{}, err
		}
		data.Details[index].AdvertiserID = client.advertiserID
	}
	return numberPage(data.Details, page, pageSize, data.TotalCount)
}

func (client *Client) CreateCreative(ctx context.Context, input CreateCreativeRequest, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_create"
	if !validID(input.UnitID) || !validRequiredText(input.Name, 100) || input.MaterialType < 0 ||
		input.ActionBarText != "" && !validRequiredText(input.ActionBarText, 100) ||
		input.Description != "" && !validRequiredText(input.Description, 30) ||
		input.PhotoID != "" && !validOpaque(input.PhotoID, 2048) || input.ImageToken != "" && !validOpaque(input.ImageToken, 8192) {
		return nil, invalidArgument(operation, "unit, name, material, or creative text is invalid")
	}
	for _, token := range input.ImageTokens {
		if !validOpaque(token, 8192) {
			return nil, invalidArgument(operation, "image tokens must be non-empty opaque values")
		}
	}
	if input.MaterialType == 0 && len(input.Fields) == 0 || input.PhotoID == "" && len(input.ImageTokens) == 0 && len(input.Fields) == 0 {
		return nil, invalidArgument(operation, "a material type and provider material reference are required")
	}
	fixed := map[string]any{"advertiser_id": client.advertiserID, "unit_id": input.UnitID, "creative_name": input.Name}
	if input.MaterialType > 0 {
		fixed["creative_material_type"] = input.MaterialType
	}
	if input.ActionBarText != "" {
		fixed["action_bar_text"] = input.ActionBarText
	}
	if input.Description != "" {
		fixed["description"] = input.Description
	}
	if input.PhotoID != "" {
		fixed["photo_id"] = input.PhotoID
	}
	if input.ImageToken != "" {
		fixed["image_token"] = input.ImageToken
	}
	if len(input.ImageTokens) > 0 {
		fixed["image_tokens"] = input.ImageTokens
	}
	body, err := mergeFields(operation, fixed, input.Fields, "creative_id", "unit_id", "creative_name", "put_status")
	if err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		CreativeID int64 `json:"creative_id"`
	}]
	header, err := client.postJSON(ctx, operation, "/gw/dsp/creative/create", body, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireResourceID(operation, 0, data.CreativeID); err != nil {
		return nil, err
	}
	return &Creative{
		ID: data.CreativeID, AdvertiserID: client.advertiserID, UnitID: input.UnitID, Name: input.Name,
		MaterialType: input.MaterialType, ActionBarText: input.ActionBarText, Description: input.Description,
		PhotoID: input.PhotoID, ImageToken: input.ImageToken, ImageTokens: append([]string(nil), input.ImageTokens...),
	}, nil
}

func (client *Client) SetCreativeStatus(ctx context.Context, creativeID int64, status PutStatus, options ...socialhub.CallOption) (BatchResult, error) {
	return client.setStatus(ctx, "creative_status_update", "/v1/creative/update/status", "creative_id", creativeID, status, options...)
}
