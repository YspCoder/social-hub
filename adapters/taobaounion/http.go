package taobaounion

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
	values url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	if err := validateCallOptions(operation, options); err != nil {
		return ResponseMeta{}, err
	}
	values.Set("method", method)
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
		return ResponseMeta{}, platformContractError(operation, "TOP returned an invalid successful response")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return ResponseMeta{}, platformContractError(operation, "TOP response must be a JSON object")
	}
	if errorRaw, found := root["error_response"]; found && len(bytes.TrimSpace(errorRaw)) > 0 && !bytes.Equal(bytes.TrimSpace(errorRaw), []byte("null")) {
		var response topErrorResponse
		if err := json.Unmarshal(errorRaw, &response); err != nil {
			return ResponseMeta{}, platformContractError(operation, "TOP returned a malformed error_response")
		}
		return ResponseMeta{}, topErrorValue(operation, metadata.StatusCode, metadata.Header, response, client.clock.Now())
	}
	responseKey := strings.ReplaceAll(strings.TrimPrefix(method, "taobao."), ".", "_") + "_response"
	responseRaw, found := root[responseKey]
	if !found {
		if _, simplified := root["request_id"]; !simplified {
			return ResponseMeta{}, platformContractError(operation, "TOP response omitted the expected method envelope")
		}
		responseRaw = raw
	}
	if len(bytes.TrimSpace(responseRaw)) == 0 || bytes.Equal(bytes.TrimSpace(responseRaw), []byte("null")) {
		return ResponseMeta{}, platformContractError(operation, "TOP success envelope is empty")
	}
	var responseMeta struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(responseRaw, &responseMeta); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	result := ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		responseMeta.RequestID, firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"),
	), 256)}
	if output != nil {
		if err := json.Unmarshal(responseRaw, output); err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return result, nil
}
