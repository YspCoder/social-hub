package googleplaces

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetPhotoMedia(ctx context.Context, input GetPhotoMediaRequest, options ...socialhub.CallOption) (PhotoMedia, error) {
	const operation = "get_photo_media"
	if !validGetPhotoMedia(input) {
		return PhotoMedia{}, invalidArgument(operation, "photo resource name or requested dimensions are invalid")
	}
	requestOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return PhotoMedia{}, err
	}
	query := make(url.Values)
	if input.MaxWidthPx > 0 {
		query.Set("maxWidthPx", strconv.Itoa(input.MaxWidthPx))
	}
	if input.MaxHeightPx > 0 {
		query.Set("maxHeightPx", strconv.Itoa(input.MaxHeightPx))
	}
	query.Set("skipHttpRedirect", "true")
	var response PhotoMedia
	meta, err := client.doJSON(ctx, operation, http.MethodGet, "/"+input.PhotoName+"/media", query, nil, &response, "", requestOptions...)
	response.Meta = meta
	if err == nil {
		expectedName := input.PhotoName + "/media"
		if !validPhotoMediaName(response.Name) || response.Name != expectedName {
			err = platformContractError(operation, "Google returned a mismatched photo media resource name")
		} else if !validHTTPSURL(response.PhotoURI) {
			err = platformContractError(operation, "Google returned an invalid or non-HTTPS photo media URI")
		}
	}
	return response, err
}
