package appleads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCreatives(ctx context.Context, pagination Pagination, options ...socialhub.CallOption) (Page[Creative], error) {
	const operation = "creatives_list"
	if !validPagination(pagination) {
		return Page[Creative]{}, invalidArgument(operation, "pagination must use offset >= 0 and limit 1..1000")
	}
	var response responseEnvelope[[]Creative]
	if err := client.getJSON(ctx, operation, "/creatives", listQuery(pagination), &response, options...); err != nil {
		return Page[Creative]{}, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return Page[Creative]{}, err
	}
	for index := range response.Data {
		if err := client.validateCreative(operation, &response.Data[index], 0); err != nil {
			return Page[Creative]{}, err
		}
	}
	return pageResult(response.Data, response.Pagination), nil
}

func (client *Client) GetCreative(ctx context.Context, creativeID int64, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_get"
	if !validID(creativeID) {
		return nil, invalidArgument(operation, "Creative ID must be positive")
	}
	var response responseEnvelope[Creative]
	if err := client.getJSON(ctx, operation, "/creatives/"+formatID(creativeID), nil, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateCreative(operation, &response.Data, creativeID); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

type creativeCreate struct {
	AdamID        int64  `json:"adamId"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	ProductPageID string `json:"productPageId,omitempty"`
}

func (client *Client) CreateCreative(ctx context.Context, input CreateCreativeRequest, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_create"
	if !validCreateCreative(input) {
		return nil, invalidArgument(operation, "Creative fields are invalid")
	}
	payload := creativeCreate{AdamID: input.AdamID, Name: input.Name, Type: input.Type, ProductPageID: input.ProductPageID}
	var response responseEnvelope[Creative]
	if err := client.postJSON(ctx, operation, "/creatives", payload, &response, options...); err != nil {
		return nil, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return nil, err
	}
	if err := client.validateCreative(operation, &response.Data, 0); err != nil {
		return nil, err
	}
	if response.Data.AdamID != input.AdamID || response.Data.Type != input.Type ||
		input.ProductPageID != "" && response.Data.ProductPageID != input.ProductPageID {
		return nil, platformContractError(operation, "Creative response did not match the requested app or product page")
	}
	return &response.Data, nil
}

func (client *Client) validateCreative(operation string, creative *Creative, expectedID int64) error {
	if creative == nil || !validID(creative.ID) || creative.OrgID != client.orgID || !validID(creative.AdamID) {
		return platformContractError(operation, "Creative response has invalid ID or organization ownership")
	}
	if expectedID != 0 && creative.ID != expectedID {
		return platformContractError(operation, "Creative response ID did not match the requested Creative")
	}
	return nil
}

func validCreateCreative(input CreateCreativeRequest) bool {
	if !validID(input.AdamID) || !validText(input.Name, 200) {
		return false
	}
	switch input.Type {
	case CreativeCustomProductPage:
		return validOpaque(input.ProductPageID, 256)
	case CreativeDefaultProductPage:
		return input.ProductPageID == ""
	default:
		return false
	}
}
