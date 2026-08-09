package ads

import (
	"bytes"
	"encoding/json"

	"social-hub/internal/transport"
)

type batchException struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type batchExceptions []batchException

func (exceptions *batchExceptions) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*exceptions = nil
		return nil
	}
	if data[0] == '[' {
		return json.Unmarshal(data, (*[]batchException)(exceptions))
	}
	var single batchException
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*exceptions = []batchException{single}
	return nil
}

type batchItem[T any] struct {
	Data       *T              `json:"data"`
	Exceptions batchExceptions `json:"exceptions"`
}

type batchResponse[T any] struct {
	Items []batchItem[T] `json:"items"`
}

func requireBatchResult[T any](operation string, response batchResponse[T], metadata transport.ResponseMetadata, validate func(*T) error) (*T, error) {
	if len(response.Items) != 1 {
		return nil, platformContractError(operation, "Pinterest did not return exactly one batch result")
	}
	item := response.Items[0]
	if len(item.Exceptions) > 0 {
		return nil, batchItemError(operation, metadata.StatusCode, metadata.Header, item.Exceptions[0])
	}
	if item.Data == nil {
		return nil, platformContractError(operation, "Pinterest batch result omitted both data and exceptions")
	}
	if err := validate(item.Data); err != nil {
		return nil, err
	}
	return item.Data, nil
}
