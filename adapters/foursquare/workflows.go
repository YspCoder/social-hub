package foursquare

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

// SearchPlaces returns one Foursquare place search page.
func (client *Client) SearchPlaces(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (*PlacePage, error) {
	const operation = "search_places"
	if err := validateSearchRequest(input); err != nil {
		return nil, err
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	fields, err := resolveFields(operation, input.Fields, callOptions.Fields)
	if err != nil {
		return nil, err
	}
	query := make(url.Values)
	setOptionalQuery(query, "query", input.Query)
	if input.LL != nil {
		query.Set("ll", formatCoordinate(*input.LL))
	}
	if input.Radius > 0 {
		query.Set("radius", strconv.Itoa(input.Radius))
	}
	setOptionalQuery(query, "near", input.Near)
	if input.NorthEast != nil {
		query.Set("ne", formatCoordinate(*input.NorthEast))
		query.Set("sw", formatCoordinate(*input.SouthWest))
	}
	if len(input.CategoryIDs) > 0 {
		query.Set("fsq_category_ids", strings.Join(input.CategoryIDs, ","))
	}
	setOptionalQuery(query, "sort", string(input.Sort))
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	setOptionalQuery(query, "cursor", input.Cursor)
	if len(fields) > 0 {
		query.Set("fields", strings.Join(fields, ","))
	}
	var response struct {
		Results []Place `json:"results"`
	}
	metadata, raw, err := client.getJSON(ctx, operation, "/places/search", query, &response, options...)
	if err != nil {
		return nil, err
	}
	maximum := input.Limit
	if maximum == 0 {
		maximum = 10
	}
	if response.Results == nil || len(response.Results) > maximum {
		return nil, platformContractError(operation, "Foursquare returned an invalid or oversized place page")
	}
	seen := make(map[string]struct{}, len(response.Results))
	for index := range response.Results {
		identifier := response.Results[index].FSQPlaceID
		if !validPlaceID(identifier) {
			return nil, platformContractError(operation, "Foursquare returned a place without a valid fsq_place_id")
		}
		if _, exists := seen[identifier]; exists {
			return nil, platformContractError(operation, "Foursquare returned a duplicate fsq_place_id")
		}
		seen[identifier] = struct{}{}
	}
	link := metadata.Header.Get("Link")
	return &PlacePage{
		Places: response.Results, NextCursor: nextCursorFromLink(link), Link: link,
		RequestID: metadata.Header.Get("X-Fsq-Request-ID"), Raw: raw,
	}, nil
}

// GetPlace returns details for one Foursquare place ID.
func (client *Client) GetPlace(ctx context.Context, fsqPlaceID string, options ...socialhub.CallOption) (*Place, error) {
	const operation = "get_place"
	if !validPlaceID(fsqPlaceID) {
		return nil, invalidArgument(operation, "fsq_place_id is invalid")
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	fields, err := resolveFields(operation, nil, callOptions.Fields)
	if err != nil {
		return nil, err
	}
	query := make(url.Values)
	if len(fields) > 0 {
		query.Set("fields", strings.Join(fields, ","))
	}
	var place Place
	metadata, _, err := client.getJSON(ctx, operation, "/places/"+fsqPlaceID, query, &place, options...)
	if err != nil {
		return nil, err
	}
	if place.FSQPlaceID == "" || place.FSQPlaceID != fsqPlaceID {
		return nil, platformContractError(operation, "Foursquare returned an absent or mismatched fsq_place_id")
	}
	place.RequestID = metadata.Header.Get("X-Fsq-Request-ID")
	return &place, nil
}

func formatCoordinate(value Coordinate) string {
	return strconv.FormatFloat(value.Latitude, 'f', -1, 64) + "," + strconv.FormatFloat(value.Longitude, 'f', -1, 64)
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

var _ PlacesWorkflow = (*Client)(nil)
