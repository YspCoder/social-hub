package googleads

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

func (client *Client) Search(ctx context.Context, input SearchRequest, options ...socialhub.CallOption) (SearchPage, error) {
	const operation = "gaql_search"
	if !validGAQL(input.Query) || !validPageToken(input.PageToken) {
		return SearchPage{}, invalidArgument(operation, "query must be one bounded SELECT statement and page token must be opaque")
	}
	response, err := searchRows[json.RawMessage](ctx, client, operation, input.Query, input.PageToken, input.ValidateOnly, options...)
	if err != nil {
		return SearchPage{}, err
	}
	return SearchPage{
		Rows: response.Results, NextPageToken: response.NextPageToken, FieldMask: response.FieldMask,
		TotalResultsCount: response.TotalResultsCount, QueryResourceConsumption: response.QueryResourceConsumption,
	}, nil
}

func searchRows[T any](ctx context.Context, client *Client, operation, query, pageToken string, validateOnly bool, options ...socialhub.CallOption) (searchResponse[T], error) {
	if !validPageToken(pageToken) {
		return searchResponse[T]{}, invalidArgument(operation, "page token is invalid")
	}
	body := map[string]any{"query": query}
	if pageToken != "" {
		body["pageToken"] = pageToken
	}
	if validateOnly {
		body["validateOnly"] = true
	}
	var response searchResponse[T]
	if _, err := client.postJSON(ctx, operation, client.searchPath(), body, &response, options...); err != nil {
		return searchResponse[T]{}, err
	}
	if !validPageToken(response.NextPageToken) || len(response.FieldMask) > 65536 ||
		!validOptionalInt64(response.TotalResultsCount) || !validOptionalInt64(response.QueryResourceConsumption) {
		return searchResponse[T]{}, platformContractError(operation, "Google Ads returned invalid Search pagination or metadata")
	}
	return response, nil
}

func validOptionalInt64(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 20 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
