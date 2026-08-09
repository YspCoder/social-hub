package oceanengine

import (
	"encoding/json"
	"net/url"
)

func setJSONQuery(query url.Values, key string, value any, operation string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return platformError(operation, "invalid_argument", "permanent", err)
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
		return platformContractError(operation, "Ocean Engine returned invalid page_info")
	}
	return nil
}

type providerMutationError struct {
	ErrorCode    int64  `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

func containsID(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
