package pddunion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) doForm(
	ctx context.Context,
	operation string,
	method string,
	responseKey string,
	values url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	if err := validateCallOptions(operation, options); err != nil {
		return ResponseMeta{}, err
	}
	values.Set("type", method)
	payload := values.Encode()
	if len(payload) > maxRequestBytes {
		return ResponseMeta{}, invalidArgument(operation, "request form exceeds 1 MiB")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, client.gatewayPath, nil, strings.NewReader(payload), options...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK || len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, platformContractError(operation, "Pinduoduo returned an invalid successful response")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return ResponseMeta{}, platformContractError(operation, "Pinduoduo response must be a JSON object")
	}
	if errorRaw, found := root["error_response"]; found && hasJSONValue(errorRaw) {
		var response pddErrorResponse
		if err := json.Unmarshal(errorRaw, &response); err != nil {
			return ResponseMeta{}, platformContractError(operation, "Pinduoduo returned a malformed error_response")
		}
		return ResponseMeta{}, pddErrorValue(operation, metadata.StatusCode, metadata.Header, response, client.clock.Now())
	}
	responseRaw, found := root[responseKey]
	if !found || !hasJSONValue(responseRaw) {
		return ResponseMeta{}, platformContractError(operation, "Pinduoduo response omitted the expected method envelope")
	}
	trimmed := bytes.TrimSpace(responseRaw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return ResponseMeta{}, platformContractError(operation, "Pinduoduo method envelope must be an object")
	}
	var responseMeta struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(trimmed, &responseMeta); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	meta := ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		responseMeta.RequestID, scalarString(root["request_id"]),
		firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"),
	), 256)}
	if output != nil {
		if err := json.Unmarshal(trimmed, output); err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return meta, nil
}

func hasJSONValue(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}
