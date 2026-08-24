package yelp

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListCategories(ctx context.Context, input ListCategoriesRequest, options ...socialhub.CallOption) (CategoriesResponse, error) {
	const operation = "list_categories"
	if !validListCategories(input) {
		return CategoriesResponse{}, invalidArgument(operation, "locale is invalid")
	}
	query := make(url.Values)
	setOptional(query, "locale", input.Locale)
	var response CategoriesResponse
	meta, err := client.getJSON(ctx, operation, "/categories", query, &response, options...)
	response.Meta = meta
	if err == nil && !validCategoriesResponse(response) {
		err = platformContractError(operation, "Yelp returned an invalid categories response")
	}
	return response, err
}
