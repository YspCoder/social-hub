package blogger

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// GetPage returns one static Blogger page by provider IDs.
func (client *Client) GetPage(ctx context.Context, input GetPageRequest, options ...socialhub.CallOption) (Page, error) {
	const operation = "get_page"
	if !validResourceID(input.BlogID) || !validResourceID(input.PageID) || !validView(input.View) {
		return Page{}, invalidArgument(operation, "blog ID, page ID, or view is invalid")
	}
	query := make(url.Values)
	setStringQuery(query, "view", string(input.View))
	var page Page
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/pages/"+input.PageID, query, &page, options...)
	if err != nil {
		return Page{}, err
	}
	page.Meta = meta
	if !validPageResponse(page, input.BlogID, input.PageID) {
		return Page{}, platformContractError(operation, "Blogger returned a page with an invalid kind, ID, or blog ownership")
	}
	return page, nil
}

// ListPages returns one provider-controlled page of static Blogger pages.
func (client *Client) ListPages(ctx context.Context, input ListPagesRequest, options ...socialhub.CallOption) (PageList, error) {
	const operation = "list_pages"
	if !validResourceID(input.BlogID) || !validPageToken(input.PageToken) ||
		!validPageStatus(input.Status) || !validView(input.View) {
		return PageList{}, invalidArgument(operation, "blog ID, pagination, status, or view is invalid")
	}
	query := make(url.Values)
	setBoolQuery(query, "fetchBodies", input.FetchBodies)
	setStringQuery(query, "pageToken", input.PageToken)
	setStringQuery(query, "status", string(input.Status))
	setUint32Query(query, "maxResults", input.MaxResults)
	setStringQuery(query, "view", string(input.View))
	var pages PageList
	meta, _, err := client.getJSON(ctx, operation, "/v3/blogs/"+input.BlogID+"/pages", query, &pages, options...)
	if err != nil {
		return PageList{}, err
	}
	pages.Meta = meta
	if !validPageListResponse(pages, input.BlogID) {
		return PageList{}, platformContractError(operation, "Blogger returned an invalid page list or pagination token")
	}
	return pages, nil
}

var _ ReadWorkflow = (*Client)(nil)
