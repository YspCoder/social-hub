package googleplaces

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetPlace(ctx context.Context, input GetPlaceRequest, options ...socialhub.CallOption) (Place, error) {
	const operation = "get_place"
	if !validGetPlace(input) {
		return Place{}, invalidArgument(operation, "place ID, language, region, or session token is invalid")
	}
	requestOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return Place{}, err
	}
	fieldMask, err := resolveFieldMask(operation, input.Fields, "", false)
	if err != nil {
		return Place{}, err
	}
	query := make(url.Values)
	setOptionalQuery(query, "languageCode", input.LanguageCode)
	setOptionalQuery(query, "regionCode", input.RegionCode)
	setOptionalQuery(query, "sessionToken", input.SessionToken)
	var response Place
	meta, err := client.doJSON(ctx, operation, http.MethodGet, "/places/"+input.PlaceID, query, nil, &response, fieldMask, requestOptions...)
	response.Meta = meta
	if err == nil {
		if err = validatePlace(operation, response); err == nil && response.ID != input.PlaceID {
			err = platformContractError(operation, "Google returned details for a different place ID")
		}
	}
	return response, err
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}
