package tripadvisor

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchLocations(ctx context.Context, input SearchLocationsRequest, options ...socialhub.CallOption) (SearchLocationsResponse, error) {
	const operation = "search_locations"
	if !validSearchRequest(input) {
		return SearchLocationsResponse{}, invalidArgument(operation, "search query, filters, coordinates, radius, or language are invalid")
	}
	query := make(url.Values)
	query.Set("searchQuery", input.SearchQuery)
	setSearchFilters(query, input.Category, input.Phone, input.Address, input.Coordinate, input.Radius, input.RadiusUnit, input.Language)
	var response SearchLocationsResponse
	meta, err := client.getJSON(ctx, operation, "/location/search", query, &response, options...)
	response.Meta = meta
	if err == nil {
		err = validateSearchResponse(operation, response)
	}
	return response, err
}

func (client *Client) SearchNearby(ctx context.Context, input SearchNearbyRequest, options ...socialhub.CallOption) (SearchLocationsResponse, error) {
	const operation = "search_nearby"
	if !validNearbyRequest(input) {
		return SearchLocationsResponse{}, invalidArgument(operation, "coordinates, filters, radius, or language are invalid")
	}
	query := make(url.Values)
	coordinate := input.Coordinate
	setSearchFilters(query, input.Category, input.Phone, input.Address, &coordinate, input.Radius, input.RadiusUnit, input.Language)
	var response SearchLocationsResponse
	meta, err := client.getJSON(ctx, operation, "/location/nearby_search", query, &response, options...)
	response.Meta = meta
	if err == nil {
		err = validateSearchResponse(operation, response)
	}
	return response, err
}

func (client *Client) GetLocationDetails(ctx context.Context, input GetLocationDetailsRequest, options ...socialhub.CallOption) (LocationDetails, error) {
	const operation = "get_location_details"
	if !validDetailsRequest(input) {
		return LocationDetails{}, invalidArgument(operation, "location ID, language, or currency is invalid")
	}
	query := make(url.Values)
	setOptional(query, "language", input.Language)
	setOptional(query, "currency", input.Currency)
	var response LocationDetails
	meta, err := client.getJSON(ctx, operation, "/location/"+input.LocationID.String()+"/details", query, &response, options...)
	response.Meta = meta
	if err == nil && response.LocationID != input.LocationID {
		err = platformContractError(operation, "Tripadvisor returned details for a different or missing location ID")
	}
	return response, err
}

func setSearchFilters(
	query url.Values,
	category Category,
	phone, address string,
	coordinate *Coordinate,
	radius *float64,
	radiusUnit RadiusUnit,
	language string,
) {
	if category != "" {
		query.Set("category", string(category))
	}
	setOptional(query, "phone", phone)
	setOptional(query, "address", address)
	if coordinate != nil {
		query.Set("latLong", formatCoordinate(*coordinate))
	}
	if radius != nil {
		query.Set("radius", strconv.FormatFloat(*radius, 'f', -1, 64))
	}
	if radiusUnit != "" {
		query.Set("radiusUnit", string(radiusUnit))
	}
	setOptional(query, "language", language)
}

func formatCoordinate(value Coordinate) string {
	return strconv.FormatFloat(value.Latitude, 'f', -1, 64) + "," +
		strconv.FormatFloat(value.Longitude, 'f', -1, 64)
}

func setOptional(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setPositiveInt(query url.Values, key string, value int) {
	if value > 0 {
		query.Set(key, strconv.Itoa(value))
	}
}

func validateSearchResponse(operation string, response SearchLocationsResponse) error {
	if response.Data == nil || len(response.Data) > MaximumSearchResults {
		return platformContractError(operation, "Tripadvisor returned an invalid or oversized search result list")
	}
	seen := make(map[ID]struct{}, len(response.Data))
	for _, location := range response.Data {
		if !validID(location.LocationID) || strings.TrimSpace(location.Name) == "" {
			return platformContractError(operation, "Tripadvisor returned a location with a missing ID or name")
		}
		if _, exists := seen[location.LocationID]; exists {
			return platformContractError(operation, "Tripadvisor returned a duplicate location ID")
		}
		seen[location.LocationID] = struct{}{}
	}
	return nil
}
