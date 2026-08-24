package itunessearch

import (
	"context"

	"social-hub/pkg/socialhub"
)

// Lookup retrieves public Store metadata by one documented identifier family.
func (client *Client) Lookup(ctx context.Context, input LookupRequest, options ...socialhub.CallOption) (*CatalogResponse, error) {
	normalized, err := normalizeLookupRequest(input)
	if err != nil {
		return nil, err
	}
	raw, meta, err := client.getJSON(ctx, "lookup", "/lookup", lookupQuery(normalized), options...)
	if err != nil {
		return nil, err
	}
	return decodeCatalogResponse("lookup", raw, meta, 0)
}
