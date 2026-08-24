package pixabay

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchImages(ctx context.Context, input ImageSearchRequest, options ...socialhub.CallOption) (ImageSearchResponse, error) {
	const operation = "search_images"
	if !validImageSearch(input) {
		return ImageSearchResponse{}, invalidArgument(operation, "query, language, image filters, dimensions, ordering, or pagination are invalid")
	}
	query := searchQuery(input.SearchRequest)
	setOptional(query, "image_type", string(input.ImageType))
	setOptional(query, "orientation", string(input.Orientation))
	if len(input.Colors) != 0 {
		colors := make([]string, len(input.Colors))
		for index, color := range input.Colors {
			colors[index] = string(color)
		}
		query.Set("colors", strings.Join(colors, ","))
	}
	var response ImageSearchResponse
	meta, _, err := client.getJSON(ctx, operation, "/", query, &response, options...)
	if err != nil {
		return ImageSearchResponse{}, err
	}
	if !validImageSearchResponse(response, input) {
		return ImageSearchResponse{}, platformContractError(operation, "Pixabay returned an invalid image search response")
	}
	meta.Page, meta.PerPage = effectivePagination(input.Page, input.PerPage)
	response.Meta = meta
	return response, nil
}

func (client *Client) SearchVideos(ctx context.Context, input VideoSearchRequest, options ...socialhub.CallOption) (VideoSearchResponse, error) {
	const operation = "search_videos"
	if !validVideoSearch(input) {
		return VideoSearchResponse{}, invalidArgument(operation, "query, language, video filters, dimensions, ordering, or pagination are invalid")
	}
	query := searchQuery(input.SearchRequest)
	setOptional(query, "video_type", string(input.VideoType))
	var response VideoSearchResponse
	meta, _, err := client.getJSON(ctx, operation, "/videos/", query, &response, options...)
	if err != nil {
		return VideoSearchResponse{}, err
	}
	if !validVideoSearchResponse(response, input) {
		return VideoSearchResponse{}, platformContractError(operation, "Pixabay returned an invalid video search response")
	}
	meta.Page, meta.PerPage = effectivePagination(input.Page, input.PerPage)
	response.Meta = meta
	return response, nil
}

func searchQuery(input SearchRequest) url.Values {
	query := make(url.Values)
	setOptional(query, "q", input.Query)
	setOptional(query, "lang", string(input.Language))
	setOptional(query, "category", string(input.Category))
	if input.MinimumWidth > 0 {
		query.Set("min_width", strconv.Itoa(input.MinimumWidth))
	}
	if input.MinimumHeight > 0 {
		query.Set("min_height", strconv.Itoa(input.MinimumHeight))
	}
	if input.EditorsChoice {
		query.Set("editors_choice", "true")
	}
	if input.SafeSearch {
		query.Set("safesearch", "true")
	}
	setOptional(query, "order", string(input.Order))
	if input.Page > 0 {
		query.Set("page", strconv.Itoa(input.Page))
	}
	if input.PerPage > 0 {
		query.Set("per_page", strconv.Itoa(input.PerPage))
	}
	return query
}

func setOptional(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

var _ CatalogWorkflow = (*Client)(nil)
