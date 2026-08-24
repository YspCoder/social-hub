package amazoncreators

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchItems(
	ctx context.Context,
	input SearchItemsRequest,
	options ...socialhub.CallOption,
) (SearchItemsResponse, error) {
	const operation = "search_items"
	if !validSearch(input) {
		return SearchItemsResponse{}, invalidArgument(operation, "search terms, filters, pagination, localization, properties, sort, or resources are invalid")
	}
	if input.ItemCount == 0 {
		input.ItemCount = 10
	}
	if input.ItemPage == 0 {
		input.ItemPage = 1
	}
	payload := struct {
		Marketplace string `json:"marketplace"`
		PartnerTag  string `json:"partnerTag"`
		SearchItemsRequest
	}{Marketplace: client.marketplace, PartnerTag: client.partnerTag, SearchItemsRequest: input}
	var output SearchItemsResponse
	meta, err := client.postJSON(ctx, operation, "/catalog/v1/searchItems", payload, &output, options...)
	if err != nil {
		return SearchItemsResponse{}, err
	}
	if output.SearchResult == nil && len(output.Errors) == 0 {
		return SearchItemsResponse{}, platformContractError(operation, "Amazon returned neither a search result nor partial errors")
	}
	output.Meta = meta
	return output, nil
}

func (client *Client) GetItems(
	ctx context.Context,
	input GetItemsRequest,
	options ...socialhub.CallOption,
) (GetItemsResponse, error) {
	const operation = "get_items"
	if !validGetItems(input) {
		return GetItemsResponse{}, invalidArgument(operation, "item IDs, localization, properties, condition, or resources are invalid")
	}
	payload := struct {
		Marketplace string `json:"marketplace"`
		PartnerTag  string `json:"partnerTag"`
		GetItemsRequest
	}{Marketplace: client.marketplace, PartnerTag: client.partnerTag, GetItemsRequest: input}
	var output GetItemsResponse
	meta, err := client.postJSON(ctx, operation, "/catalog/v1/getItems", payload, &output, options...)
	if err != nil {
		return GetItemsResponse{}, err
	}
	if output.ItemsResult == nil && len(output.Errors) == 0 {
		return GetItemsResponse{}, platformContractError(operation, "Amazon returned neither an items result nor partial errors")
	}
	output.Meta = meta
	return output, nil
}

func (client *Client) GetVariations(
	ctx context.Context,
	input GetVariationsRequest,
	options ...socialhub.CallOption,
) (GetVariationsResponse, error) {
	const operation = "get_variations"
	if !validGetVariations(input) {
		return GetVariationsResponse{}, invalidArgument(operation, "ASIN, pagination, localization, properties, condition, or resources are invalid")
	}
	if input.VariationCount == 0 {
		input.VariationCount = 10
	}
	if input.VariationPage == 0 {
		input.VariationPage = 1
	}
	payload := struct {
		Marketplace string `json:"marketplace"`
		PartnerTag  string `json:"partnerTag"`
		GetVariationsRequest
	}{Marketplace: client.marketplace, PartnerTag: client.partnerTag, GetVariationsRequest: input}
	var output GetVariationsResponse
	meta, err := client.postJSON(ctx, operation, "/catalog/v1/getVariations", payload, &output, options...)
	if err != nil {
		return GetVariationsResponse{}, err
	}
	if output.VariationsResult == nil && len(output.Errors) == 0 {
		return GetVariationsResponse{}, platformContractError(operation, "Amazon returned neither a variations result nor partial errors")
	}
	output.Meta = meta
	return output, nil
}

func (client *Client) GetBrowseNodes(
	ctx context.Context,
	input GetBrowseNodesRequest,
	options ...socialhub.CallOption,
) (GetBrowseNodesResponse, error) {
	const operation = "get_browse_nodes"
	if !validGetBrowseNodes(input) {
		return GetBrowseNodesResponse{}, invalidArgument(operation, "browse node IDs, localization, or resources are invalid")
	}
	payload := struct {
		Marketplace string `json:"marketplace"`
		PartnerTag  string `json:"partnerTag"`
		GetBrowseNodesRequest
	}{Marketplace: client.marketplace, PartnerTag: client.partnerTag, GetBrowseNodesRequest: input}
	var output GetBrowseNodesResponse
	meta, err := client.postJSON(ctx, operation, "/catalog/v1/getBrowseNodes", payload, &output, options...)
	if err != nil {
		return GetBrowseNodesResponse{}, err
	}
	if output.BrowseNodesResult == nil && len(output.Errors) == 0 {
		return GetBrowseNodesResponse{}, platformContractError(operation, "Amazon returned neither a Browse Nodes result nor partial errors")
	}
	output.Meta = meta
	return output, nil
}

var _ CatalogWorkflow = (*Client)(nil)
