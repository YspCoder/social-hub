package tidal

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// Search returns TIDAL's searchResults collection and any requested album,
// artist, or track resources in Included.
func (client *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (*Page[SearchResult], error) {
	const operation = "search"
	if err := validateSearchRequest(input); err != nil {
		return nil, err
	}
	query := make(url.Values)
	query.Set("filter[query]", input.Query)
	if input.ExplicitFilter != "" {
		query.Set("explicitFilter", string(input.ExplicitFilter))
	}
	addCommonQuery(query, input.CountryCode, input.Include)
	body, response, err := client.get(ctx, operation, "/searchResults", query, options...)
	if err != nil {
		return nil, err
	}
	return decodePage(operation, "/searchResults", body, response, "searchResults", func(resource *SearchResult) (string, string) {
		return resource.Type, resource.ID
	})
}
