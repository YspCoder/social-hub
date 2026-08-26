package tripadvisor

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListPhotos(ctx context.Context, input ListPhotosRequest, options ...socialhub.CallOption) (PhotoPage, error) {
	const operation = "list_photos"
	if !validPhotosRequest(input) {
		return PhotoPage{}, invalidArgument(operation, "location ID, language, pagination, or photo sources are invalid")
	}
	query := make(url.Values)
	setOptional(query, "language", input.Language)
	setPositiveInt(query, "limit", input.Limit)
	setPositiveInt(query, "offset", input.Offset)
	if len(input.Sources) > 0 {
		sources := make([]string, len(input.Sources))
		for index, source := range input.Sources {
			sources[index] = string(source)
		}
		query.Set("source", strings.Join(sources, ","))
	}
	var response PhotoPage
	meta, err := client.getJSON(ctx, operation, "/location/"+input.LocationID.String()+"/photos", query, &response, options...)
	response.Meta = meta
	response.LocationID = input.LocationID
	if err == nil {
		maximum := input.Limit
		if maximum == 0 {
			maximum = MaximumPageSize
		}
		if response.Data == nil || len(response.Data) > maximum {
			return response, platformContractError(operation, "Tripadvisor returned an invalid or oversized photo page")
		}
		seen := make(map[ID]struct{}, len(response.Data))
		for _, photo := range response.Data {
			if !validID(photo.ID) {
				return response, platformContractError(operation, "Tripadvisor returned a photo with a missing ID")
			}
			if _, exists := seen[photo.ID]; exists {
				return response, platformContractError(operation, "Tripadvisor returned a duplicate photo ID")
			}
			seen[photo.ID] = struct{}{}
		}
	}
	return response, err
}
