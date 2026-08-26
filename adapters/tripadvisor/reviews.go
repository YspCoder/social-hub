package tripadvisor

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListReviews(ctx context.Context, input ListReviewsRequest, options ...socialhub.CallOption) (ReviewPage, error) {
	const operation = "list_reviews"
	if !validReviewsRequest(input) {
		return ReviewPage{}, invalidArgument(operation, "location ID, language, or pagination is invalid")
	}
	query := make(url.Values)
	setOptional(query, "language", input.Language)
	setPositiveInt(query, "limit", input.Limit)
	setPositiveInt(query, "offset", input.Offset)
	var response ReviewPage
	meta, err := client.getJSON(ctx, operation, "/location/"+input.LocationID.String()+"/reviews", query, &response, options...)
	response.Meta = meta
	response.LocationID = input.LocationID
	if err == nil {
		maximum := input.Limit
		if maximum == 0 {
			maximum = MaximumPageSize
		}
		if response.Data == nil || len(response.Data) > maximum {
			return response, platformContractError(operation, "Tripadvisor returned an invalid or oversized review page")
		}
		seen := make(map[ID]struct{}, len(response.Data))
		for _, review := range response.Data {
			if !validID(review.ID) || review.LocationID != input.LocationID {
				return response, platformContractError(operation, "Tripadvisor returned a review with a missing ID or a different location ID")
			}
			if _, exists := seen[review.ID]; exists {
				return response, platformContractError(operation, "Tripadvisor returned a duplicate review ID")
			}
			seen[review.ID] = struct{}{}
		}
	}
	return response, err
}
