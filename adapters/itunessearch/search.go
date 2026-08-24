package itunessearch

import (
	"context"

	"social-hub/pkg/socialhub"
)

// Search queries public music, podcast, movie, audiobook, software, ebook, and
// other Store catalog metadata using Apple's documented media combinations.
func (client *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (*CatalogResponse, error) {
	normalized, err := normalizeSearchRequest(input)
	if err != nil {
		return nil, err
	}
	raw, meta, err := client.getJSON(ctx, "search", "/search", searchQuery(normalized), options...)
	if err != nil {
		return nil, err
	}
	return decodeCatalogResponse("search", raw, meta, normalized.Limit)
}
