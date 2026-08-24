package openverse

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchImages(ctx context.Context, input ImageSearchRequest, options ...socialhub.CallOption) (ImageSearchResponse, error) {
	const operation = "search_images"
	if !validImageSearch(input, client.authenticated) {
		return ImageSearchResponse{}, invalidArgument(operation, "search terms, filters, or pagination are invalid for the current authentication mode")
	}
	query := searchQuery(input.SearchRequest)
	setOptional(query, "category", string(input.Category))
	setOptional(query, "aspect_ratio", string(input.AspectRatio))
	setOptional(query, "size", string(input.Size))
	var response ImageSearchResponse
	meta, _, err := client.getJSON(ctx, operation, "/images/", query, &response, options...)
	if err != nil {
		return ImageSearchResponse{}, err
	}
	if !validImageSearchResponse(response, input) {
		return ImageSearchResponse{}, platformContractError(operation, "Openverse returned an invalid image search response")
	}
	response.Meta = meta
	return response, nil
}

func (client *Client) GetImage(ctx context.Context, identifier string, options ...socialhub.CallOption) (Image, error) {
	const operation = "get_image"
	if !validUUID(identifier) {
		return Image{}, invalidArgument(operation, "image ID must be a canonical UUID")
	}
	var response Image
	meta, _, err := client.getJSON(ctx, operation, "/images/"+identifier+"/", nil, &response, options...)
	if err != nil {
		return Image{}, err
	}
	if !validImage(response) || !strings.EqualFold(response.ID, identifier) {
		return Image{}, platformContractError(operation, "Openverse returned an invalid or mismatched image")
	}
	response.Meta = meta
	return response, nil
}

func (client *Client) SearchAudio(ctx context.Context, input AudioSearchRequest, options ...socialhub.CallOption) (AudioSearchResponse, error) {
	const operation = "search_audio"
	if !validAudioSearch(input, client.authenticated) {
		return AudioSearchResponse{}, invalidArgument(operation, "search terms, filters, or pagination are invalid for the current authentication mode")
	}
	query := searchQuery(input.SearchRequest)
	setOptional(query, "category", string(input.Category))
	setOptional(query, "length", string(input.Length))
	var response AudioSearchResponse
	meta, _, err := client.getJSON(ctx, operation, "/audio/", query, &response, options...)
	if err != nil {
		return AudioSearchResponse{}, err
	}
	if !validAudioSearchResponse(response, input) {
		return AudioSearchResponse{}, platformContractError(operation, "Openverse returned an invalid audio search response")
	}
	response.Meta = meta
	return response, nil
}

func (client *Client) GetAudio(ctx context.Context, identifier string, options ...socialhub.CallOption) (Audio, error) {
	const operation = "get_audio"
	if !validUUID(identifier) {
		return Audio{}, invalidArgument(operation, "audio ID must be a canonical UUID")
	}
	var response Audio
	meta, _, err := client.getJSON(ctx, operation, "/audio/"+identifier+"/", nil, &response, options...)
	if err != nil {
		return Audio{}, err
	}
	if !validAudio(response) || !strings.EqualFold(response.ID, identifier) {
		return Audio{}, platformContractError(operation, "Openverse returned invalid or mismatched audio")
	}
	response.Meta = meta
	return response, nil
}

func searchQuery(input SearchRequest) url.Values {
	query := make(url.Values)
	setOptional(query, "q", input.Query)
	setOptionalList(query, "source", input.Sources)
	setOptionalList(query, "excluded_source", input.ExcludedSources)
	setOptionalTypedList(query, "license", input.Licenses)
	setOptionalTypedList(query, "license_type", input.LicenseTypes)
	setOptional(query, "creator", input.Creator)
	setOptional(query, "tags", input.Tags)
	setOptional(query, "title", input.Title)
	setOptional(query, "extension", input.Extension)
	if input.Mature != nil {
		query.Set("mature", strconv.FormatBool(*input.Mature))
	}
	if input.Page > 0 {
		query.Set("page", strconv.Itoa(input.Page))
	}
	if input.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(input.PageSize))
	}
	return query
}

func setOptional(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalList(query url.Values, key string, values []string) {
	if len(values) != 0 {
		query.Set(key, strings.Join(values, ","))
	}
}

func setOptionalTypedList[T ~string](query url.Values, key string, values []T) {
	if len(values) == 0 {
		return
	}
	items := make([]string, len(values))
	for index, value := range values {
		items[index] = string(value)
	}
	query.Set(key, strings.Join(items, ","))
}

var _ ImagesWorkflow = (*Client)(nil)
var _ AudioWorkflow = (*Client)(nil)
