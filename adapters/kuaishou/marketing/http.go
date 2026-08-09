package marketing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

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
	request.Header.Set("Accept", "application/json")
	metadata, err := client.api.DoWithMetadata(request, output)
	return metadata.Header, err
}

func numberPage[T any](items []T, page, pageSize int, total int64) (NumberPage[T], error) {
	if total < 0 || int64(len(items)) > total {
		return NumberPage[T]{}, platformContractError("pagination", "Kuaishou returned invalid pagination metadata")
	}
	return NumberPage[T]{
		Items: items, Page: page, PageSize: pageSize, TotalNumber: total,
		HasMore: int64(page*pageSize) < total,
	}, nil
}

func requireAdvertiser(operation string, expected, actual int64) error {
	if actual != 0 && actual != expected {
		return platformContractError(operation, "Kuaishou returned a resource for another advertiser")
	}
	return nil
}

func requireResourceID(operation string, expected, actual int64) error {
	if !validID(actual) || expected != 0 && actual != expected {
		return platformContractError(operation, "Kuaishou returned an invalid or mismatched resource ID")
	}
	return nil
}

func containsID(values []int64, expected int64) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func jsonRaw(value any) (json.RawMessage, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}
