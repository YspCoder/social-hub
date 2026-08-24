package marketing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(ctx context.Context, _ string, path string, query url.Values, output any, options ...socialhub.CallOption) (http.Header, error) {
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return nil, err
	}
	metadata, err := client.api.DoWithMetadata(request, output)
	return metadata.Header, err
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) (http.Header, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.api.DoWithMetadata(request, output)
	return metadata.Header, err
}

func setJSONQuery(query url.Values, key string, value any, operation string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query.Set(key, string(encoded))
	return nil
}

func numberPage[T any](operation string, items []T, info *pageInfo) (NumberPage[T], error) {
	if info == nil || info.Page < 1 || info.PageSize < 1 || info.TotalNumber < 0 || info.TotalPage < 0 ||
		int64(len(items)) > info.TotalNumber {
		return NumberPage[T]{}, platformContractError(operation, "TikTok returned invalid pagination metadata")
	}
	return NumberPage[T]{
		Items: items, Page: info.Page, PageSize: info.PageSize, TotalNumber: info.TotalNumber,
		TotalPage: info.TotalPage, HasMore: info.HasMore,
	}, nil
}

func requireAdvertiser(operation, expected, actual string) error {
	if actual != "" && actual != expected {
		return platformContractError(operation, "TikTok returned a resource for another advertiser")
	}
	return nil
}

func requireResourceID(operation, expected, actual string) error {
	if !validID(actual) || expected != "" && actual != expected {
		return platformContractError(operation, "TikTok returned an invalid or mismatched resource ID")
	}
	return nil
}

func containsID(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
