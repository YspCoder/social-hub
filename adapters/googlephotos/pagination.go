package googlephotos

import (
	"bytes"
	"encoding/json"
)

func decodePage[T any](
	operation string,
	field string,
	maximum int,
	meta ResponseMeta,
	raw json.RawMessage,
	valid func(T) bool,
	entityName string,
) (*Page[T], error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, platformContractError(operation, "Google Photos returned an invalid pagination envelope")
	}
	var items []T
	if encoded, exists := envelope[field]; exists {
		trimmed := bytes.TrimSpace(encoded)
		if len(trimmed) == 0 || trimmed[0] != '[' || !json.Valid(trimmed) ||
			json.Unmarshal(trimmed, &items) != nil || len(items) > maximum {
			return nil, platformContractError(operation, "Google Photos returned invalid "+entityName+" values")
		}
	}
	for _, item := range items {
		if !valid(item) {
			return nil, platformContractError(operation, "Google Photos returned a "+entityName+" without a valid id")
		}
	}
	var nextPageToken string
	if encoded, exists := envelope["nextPageToken"]; exists {
		trimmed := bytes.TrimSpace(encoded)
		if len(trimmed) == 0 || trimmed[0] != '"' || json.Unmarshal(trimmed, &nextPageToken) != nil || !validPageToken(nextPageToken) {
			return nil, platformContractError(operation, "Google Photos returned an invalid nextPageToken")
		}
	}
	return &Page[T]{
		Items: items, NextPageToken: nextPageToken, Meta: meta,
		Raw: append(json.RawMessage(nil), raw...),
	}, nil
}
