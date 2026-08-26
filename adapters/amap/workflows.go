package amap

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchText(ctx context.Context, input TextSearchRequest, options ...socialhub.CallOption) (PlacePage, error) {
	const operation = "search_text"
	if err := validateTextSearch(input); err != nil {
		return PlacePage{}, err
	}
	query := url.Values{"output": {"json"}}
	setSearchQuery(query, input.Keywords, input.TypeCodes, input.Region, input.Language, input.CityLimit, input.ShowFields, input.PageSize, input.PageNumber)
	result, err := client.getPlaces(ctx, operation, "/v5/place/text", query, options...)
	if err != nil {
		return PlacePage{}, err
	}
	if err := validateSearchResponse(operation, result); err != nil {
		return PlacePage{}, err
	}
	result.Meta.PageSize = effectivePageSize(input.PageSize)
	result.Meta.PageNumber = effectivePageNumber(input.PageNumber)
	return PlacePage{Places: result.Places, Meta: result.Meta, Raw: result.Raw}, nil
}

func (client *Client) SearchAround(ctx context.Context, input AroundSearchRequest, options ...socialhub.CallOption) (PlacePage, error) {
	const operation = "search_around"
	if err := validateAroundSearch(input); err != nil {
		return PlacePage{}, err
	}
	query := url.Values{"output": {"json"}, "location": {input.Location.String()}}
	setSearchQuery(query, input.Keywords, input.TypeCodes, input.Region, input.Language, input.CityLimit, input.ShowFields, input.PageSize, input.PageNumber)
	if input.Radius != 0 {
		query.Set("radius", strconv.Itoa(input.Radius))
	}
	if input.Sort != "" {
		query.Set("sortrule", string(input.Sort))
	}
	result, err := client.getPlaces(ctx, operation, "/v5/place/around", query, options...)
	if err != nil {
		return PlacePage{}, err
	}
	if err := validateSearchResponse(operation, result); err != nil {
		return PlacePage{}, err
	}
	result.Meta.PageSize = effectivePageSize(input.PageSize)
	result.Meta.PageNumber = effectivePageNumber(input.PageNumber)
	return PlacePage{Places: result.Places, Meta: result.Meta, Raw: result.Raw}, nil
}

func (client *Client) GetDetails(ctx context.Context, input DetailRequest, options ...socialhub.CallOption) (PlaceDetails, error) {
	const operation = "get_details"
	if err := validateDetail(input); err != nil {
		return PlaceDetails{}, err
	}
	query := url.Values{"output": {"json"}, "id": {strings.Join(input.IDs, "|")}}
	setLanguageAndFields(query, input.Language, input.ShowFields)
	result, err := client.getPlaces(ctx, operation, "/v5/place/detail", query, options...)
	if err != nil {
		return PlaceDetails{}, err
	}
	if err := validatePlaces(operation, result.Places); err != nil {
		return PlaceDetails{}, err
	}
	requested := make(map[string]struct{}, len(input.IDs))
	for _, id := range input.IDs {
		requested[id] = struct{}{}
	}
	for _, place := range result.Places {
		if _, found := requested[place.ID]; !found {
			return PlaceDetails{}, platformContractError(operation, "Amap returned an unrequested POI")
		}
	}
	return PlaceDetails{Places: result.Places, Meta: result.Meta, Raw: result.Raw}, nil
}

func setSearchQuery(query url.Values, keywords string, typeCodes []string, region string, language Language, cityLimit *bool, showFields []ShowField, pageSize, pageNumber int) {
	if keywords != "" {
		query.Set("keywords", keywords)
	}
	if len(typeCodes) > 0 {
		query.Set("types", strings.Join(typeCodes, "|"))
	}
	if region != "" {
		query.Set("region", region)
	}
	if cityLimit != nil {
		query.Set("city_limit", strconv.FormatBool(*cityLimit))
	}
	setLanguageAndFields(query, language, showFields)
	if pageSize != 0 {
		query.Set("page_size", strconv.Itoa(pageSize))
	}
	if pageNumber != 0 {
		query.Set("page_num", strconv.Itoa(pageNumber))
	}
}

func setLanguageAndFields(query url.Values, language Language, fields []ShowField) {
	if language != "" {
		query.Set("langCode", string(language))
	}
	if len(fields) > 0 {
		values := make([]string, len(fields))
		for index, field := range fields {
			values[index] = string(field)
		}
		query.Set("show_fields", strings.Join(values, ","))
	}
}

func validateSearchResponse(operation string, result providerResponse) error {
	if !result.CountPresent || result.Meta.Count < len(result.Places) {
		return platformContractError(operation, "Amap returned an inconsistent search count")
	}
	return validatePlaces(operation, result.Places)
}

func validatePlaces(operation string, places []Place) error {
	if places == nil {
		return platformContractError(operation, "Amap returned a null POI list")
	}
	seen := make(map[string]struct{}, len(places))
	for _, place := range places {
		if !validOpaque(place.ID, 128) || place.Name == "" || place.Location == nil || !validCoordinate(*place.Location) {
			return platformContractError(operation, "Amap returned an invalid POI")
		}
		if _, exists := seen[place.ID]; exists {
			return platformContractError(operation, "Amap returned a duplicate POI ID")
		}
		seen[place.ID] = struct{}{}
	}
	return nil
}
