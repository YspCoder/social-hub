package tencentads

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) requestJSON(ctx context.Context, operation, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) (http.Header, error) {
	var body *bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return nil, err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
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

type pageInfo struct {
	Page        int   `json:"page"`
	PageSize    int   `json:"page_size"`
	TotalNumber int64 `json:"total_number"`
	TotalPage   int   `json:"total_page"`
}

func numberPage[T any](items []T, info *pageInfo) NumberPage[T] {
	result := NumberPage[T]{Items: items}
	if info == nil {
		return result
	}
	result.Page, result.PageSize = info.Page, info.PageSize
	result.TotalNumber, result.TotalPages = info.TotalNumber, info.TotalPage
	result.HasMore = info.Page > 0 && info.TotalPage > info.Page
	return result
}

func validatePageInfo(operation string, info *pageInfo) error {
	if info == nil || info.Page < 1 || info.PageSize < 1 || info.TotalNumber < 0 || info.TotalPage < 0 ||
		info.TotalNumber > 0 && info.TotalPage == 0 {
		return platformContractError(operation, "Tencent Ads returned invalid page_info")
	}
	return nil
}

func requireAccount(operation string, expected, actual int64) error {
	if actual != 0 && actual != expected {
		return platformContractError(operation, "Tencent Ads returned a resource for another advertiser account")
	}
	return nil
}

func requireResourceID(operation string, expected, actual int64) error {
	if !validID(actual) || expected != 0 && actual != expected {
		return platformContractError(operation, "Tencent Ads returned an invalid or mismatched resource ID")
	}
	return nil
}
