package wikimedia

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchPages(
	ctx context.Context,
	input SearchPagesRequest,
	options ...socialhub.CallOption,
) (SearchResponse, error) {
	const operation = "search_pages"
	if !validSearch(input) {
		return SearchResponse{}, invalidArgument(operation, "query must be non-empty and limit must be omitted or between 1 and 100")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	query := url.Values{"q": {input.Query}, "limit": {strconv.Itoa(limit)}}
	var output SearchResponse
	metadata, raw, err := client.getJSON(ctx, operation, "/v1/search/page", query, &output, options...)
	if err != nil {
		return SearchResponse{}, err
	}
	if output.Pages == nil {
		return SearchResponse{}, platformContractError(operation, "Wikimedia omitted the search pages array")
	}
	if len(output.Pages) > limit {
		return SearchResponse{}, platformContractError(operation, "Wikimedia returned more search results than requested")
	}
	pageIDs := make(map[int64]struct{}, len(output.Pages))
	for _, page := range output.Pages {
		if page.ID <= 0 {
			return SearchResponse{}, platformContractError(operation, "Wikimedia returned a search result without a valid page ID")
		}
		if _, duplicate := pageIDs[page.ID]; duplicate {
			return SearchResponse{}, platformContractError(operation, "Wikimedia returned duplicate page IDs in search results")
		}
		pageIDs[page.ID] = struct{}{}
	}
	output.Meta = metadata
	output.Raw = raw
	return output, nil
}

func (client *Client) GetPage(
	ctx context.Context,
	title string,
	input GetPageRequest,
	options ...socialhub.CallOption,
) (Page, error) {
	const operation = "get_page"
	if !validTitle(title) {
		return Page{}, invalidArgument(operation, "page title is invalid")
	}
	query := make(url.Values)
	if input.FollowRedirects != nil {
		query.Set("redirect", strconv.FormatBool(*input.FollowRedirects))
	}
	var output Page
	metadata, raw, err := client.getJSON(
		ctx, operation, "/v1/page/"+escapedTitle(title)+"/bare", query, &output, options...,
	)
	if err != nil {
		return Page{}, err
	}
	if output.ID <= 0 || output.Title == "" {
		return Page{}, platformContractError(operation, "Wikimedia returned a page without an ID or title")
	}
	output.Meta = metadata
	output.Raw = raw
	return output, nil
}

func (client *Client) ListPageMedia(
	ctx context.Context,
	title string,
	options ...socialhub.CallOption,
) (PageMediaResponse, error) {
	const operation = "list_page_media"
	if !validTitle(title) {
		return PageMediaResponse{}, invalidArgument(operation, "page title is invalid")
	}
	var output PageMediaResponse
	metadata, raw, err := client.getJSON(
		ctx, operation, "/v1/page/"+escapedTitle(title)+"/links/media", nil, &output, options...,
	)
	if err != nil {
		return PageMediaResponse{}, err
	}
	if output.Files == nil {
		return PageMediaResponse{}, platformContractError(operation, "Wikimedia omitted the page media array")
	}
	if len(output.Files) > 100 {
		return PageMediaResponse{}, platformContractError(operation, "Wikimedia returned more than 100 page media files")
	}
	fileTitles := make(map[string]struct{}, len(output.Files))
	for _, file := range output.Files {
		if !validFileTitle(file.Title) {
			return PageMediaResponse{}, platformContractError(operation, "Wikimedia returned page media without a valid file title")
		}
		if _, duplicate := fileTitles[file.Title]; duplicate {
			return PageMediaResponse{}, platformContractError(operation, "Wikimedia returned duplicate file titles in page media")
		}
		fileTitles[file.Title] = struct{}{}
	}
	output.Meta = metadata
	output.Raw = raw
	return output, nil
}

func (client *Client) GetFile(
	ctx context.Context,
	title string,
	options ...socialhub.CallOption,
) (File, error) {
	const operation = "get_file"
	if !validFileTitle(title) {
		return File{}, invalidArgument(operation, "file title must start with File:")
	}
	var output File
	metadata, raw, err := client.getJSON(
		ctx, operation, "/v1/file/"+escapedTitle(title), nil, &output, options...,
	)
	if err != nil {
		return File{}, err
	}
	if output.Title == "" {
		return File{}, platformContractError(operation, "Wikimedia returned a file without a title")
	}
	output.Meta = metadata
	output.Raw = raw
	return output, nil
}

func (client *Client) ListFileThumbnails(
	ctx context.Context,
	title string,
	options ...socialhub.CallOption,
) (FileThumbnailsResponse, error) {
	const operation = "list_file_thumbnails"
	if !validFileTitle(title) {
		return FileThumbnailsResponse{}, invalidArgument(operation, "file title must start with File:")
	}
	var output FileThumbnailsResponse
	metadata, raw, err := client.getJSON(
		ctx, operation, "/v1/file/"+escapedTitle(title)+"/thumbnails", nil, &output, options...,
	)
	if err != nil {
		return FileThumbnailsResponse{}, err
	}
	if !validFileTitle(output.Title) || output.Thumbnails == nil {
		return FileThumbnailsResponse{}, platformContractError(operation, "Wikimedia omitted the file title or thumbnails array")
	}
	if !validProviderURL(output.Original.URL) || !validOptionalDimension(output.Original.Width) || !validOptionalDimension(output.Original.Height) {
		return FileThumbnailsResponse{}, platformContractError(operation, "Wikimedia returned invalid original file dimensions or URL")
	}
	for _, thumbnail := range output.Thumbnails {
		if thumbnail.Width <= 0 || thumbnail.Height <= 0 || !validProviderURL(thumbnail.URL) {
			return FileThumbnailsResponse{}, platformContractError(operation, "Wikimedia returned invalid thumbnail dimensions or URL")
		}
		for _, responsiveURL := range thumbnail.ResponsiveURLs {
			if !validProviderURL(responsiveURL) {
				return FileThumbnailsResponse{}, platformContractError(operation, "Wikimedia returned an invalid responsive thumbnail URL")
			}
		}
	}
	output.Meta = metadata
	output.Raw = raw
	return output, nil
}

var _ KnowledgeWorkflow = (*Client)(nil)
