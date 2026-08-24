package yelp

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchBusinesses(ctx context.Context, input SearchBusinessesRequest, options ...socialhub.CallOption) (SearchBusinessesResponse, error) {
	const operation = "search_businesses"
	if !validSearchBusinesses(input) {
		return SearchBusinessesResponse{}, invalidArgument(operation, "search requires either location or complete coordinates and all filters must satisfy Yelp's documented bounds")
	}
	query := make(url.Values)
	if input.Location != "" {
		query.Set("location", input.Location)
	} else {
		query.Set("latitude", strconv.FormatFloat(*input.Latitude, 'f', -1, 64))
		query.Set("longitude", strconv.FormatFloat(*input.Longitude, 'f', -1, 64))
	}
	setOptional(query, "term", input.Term)
	if input.Radius != nil {
		query.Set("radius", strconv.Itoa(*input.Radius))
	}
	if len(input.Categories) > 0 {
		query.Set("categories", strings.Join(input.Categories, ","))
	}
	setOptional(query, "locale", input.Locale)
	if len(input.Price) > 0 {
		prices := make([]string, len(input.Price))
		for index, value := range input.Price {
			prices[index] = strconv.Itoa(value)
		}
		query.Set("price", strings.Join(prices, ","))
	}
	if input.OpenNow != nil {
		query.Set("open_now", strconv.FormatBool(*input.OpenNow))
	}
	if input.OpenAt != nil {
		query.Set("open_at", strconv.FormatInt(*input.OpenAt, 10))
	}
	if len(input.Attributes) > 0 {
		attributes := make([]string, len(input.Attributes))
		for index, value := range input.Attributes {
			attributes[index] = string(value)
		}
		query.Set("attributes", strings.Join(attributes, ","))
	}
	if input.SortBy != "" {
		query.Set("sort_by", string(input.SortBy))
	}
	setPositiveInt(query, "limit", input.Limit)
	setPositiveInt(query, "offset", input.Offset)

	var response SearchBusinessesResponse
	meta, err := client.getJSON(ctx, operation, "/businesses/search", query, &response, options...)
	response.Meta = meta
	if err == nil && !validSearchBusinessesResponse(response) {
		err = platformContractError(operation, "Yelp returned an invalid business search response")
	}
	return response, err
}

func (client *Client) GetBusiness(ctx context.Context, input GetBusinessRequest, options ...socialhub.CallOption) (Business, error) {
	const operation = "get_business"
	if !validGetBusiness(input) {
		return Business{}, invalidArgument(operation, "business ID or alias and locale are invalid")
	}
	query := make(url.Values)
	setOptional(query, "locale", input.Locale)
	var response Business
	meta, err := client.getJSON(ctx, operation, "/businesses/"+input.BusinessIDOrAlias, query, &response, options...)
	response.Meta = meta
	if err == nil && !validBusiness(response) {
		err = platformContractError(operation, "Yelp returned an invalid business response")
	}
	return response, err
}

func (client *Client) ListReviews(ctx context.Context, input ListReviewsRequest, options ...socialhub.CallOption) (ReviewsResponse, error) {
	const operation = "list_reviews"
	if !validListReviews(input) {
		return ReviewsResponse{}, invalidArgument(operation, "business ID or alias, locale, limit, or offset is invalid")
	}
	query := make(url.Values)
	setOptional(query, "locale", input.Locale)
	setPositiveInt(query, "offset", input.Offset)
	setPositiveInt(query, "limit", input.Limit)
	var response ReviewsResponse
	meta, err := client.getJSON(ctx, operation, "/businesses/"+input.BusinessIDOrAlias+"/reviews", query, &response, options...)
	response.Meta = meta
	if err == nil && !validReviewsResponse(response) {
		err = platformContractError(operation, "Yelp returned an invalid reviews response")
	}
	return response, err
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
