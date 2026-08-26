package googleplaces

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) TextSearch(ctx context.Context, input TextSearchRequest, options ...socialhub.CallOption) (PlacePage, error) {
	const operation = "text_search"
	if !validTextSearch(input) {
		return PlacePage{}, invalidArgument(operation, "text query, filters, location, pagination, language, or region is invalid")
	}
	requestOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return PlacePage{}, err
	}
	fieldMask, err := resolveFieldMask(operation, input.Fields, "places.", input.IncludeNextPageToken)
	if err != nil {
		return PlacePage{}, err
	}
	var response PlacePage
	meta, err := client.doJSON(ctx, operation, http.MethodPost, "/places:searchText", nil, input, &response, fieldMask, requestOptions...)
	response.Meta = meta
	if err == nil {
		err = validatePlacePage(operation, &response, input.PageSize)
	}
	return response, err
}

func (client *Client) NearbySearch(ctx context.Context, input NearbySearchRequest, options ...socialhub.CallOption) (PlacePage, error) {
	const operation = "nearby_search"
	if !validNearbySearch(input) {
		return PlacePage{}, invalidArgument(operation, "types, location restriction, result count, rank, language, or region is invalid")
	}
	requestOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return PlacePage{}, err
	}
	fieldMask, err := resolveFieldMask(operation, input.Fields, "places.", false)
	if err != nil {
		return PlacePage{}, err
	}
	var response PlacePage
	meta, err := client.doJSON(ctx, operation, http.MethodPost, "/places:searchNearby", nil, input, &response, fieldMask, requestOptions...)
	response.Meta = meta
	if err == nil {
		err = validatePlacePage(operation, &response, input.MaxResultCount)
	}
	return response, err
}

func validatePlacePage(operation string, response *PlacePage, requestedMaximum int) error {
	maximum := requestedMaximum
	if maximum == 0 {
		maximum = MaximumSearchPageSize
	}
	if len(response.Places) > maximum {
		return platformContractError(operation, "Google returned more places than the requested page bound")
	}
	if response.NextPageToken != "" && !validOpaque(response.NextPageToken, 8192) {
		return platformContractError(operation, "Google returned an invalid next page token")
	}
	for index := range response.Places {
		if err := validatePlace(operation, response.Places[index]); err != nil {
			return err
		}
		response.Places[index].Meta = response.Meta
	}
	return nil
}

var _ PlacesWorkflow = (*Client)(nil)
