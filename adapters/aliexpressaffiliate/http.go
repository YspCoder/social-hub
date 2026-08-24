package aliexpressaffiliate

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
	payload := values.Encode()
	if len(payload) > maxRequestBytes {
		return ResponseMeta{}, invalidArgument(operation, "request form exceeds 1 MiB")
	}
	route := make(url.Values)
	route.Set("method", method)
	request, err := client.api.NewRequest(ctx, http.MethodPost, client.gatewayPath, route, strings.NewReader(payload), options...)
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
		return ResponseMeta{}, platformContractError(operation, "AliExpress returned an invalid successful response")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return ResponseMeta{}, platformContractError(operation, "AliExpress response must be a JSON object")
	}
	if errorRaw, found := root["error_response"]; found && len(bytes.TrimSpace(errorRaw)) > 0 && !bytes.Equal(bytes.TrimSpace(errorRaw), []byte("null")) {
		var response topErrorResponse
		if err := json.Unmarshal(errorRaw, &response); err != nil {
			return ResponseMeta{}, platformContractError(operation, "AliExpress returned a malformed error_response")
		}
		return ResponseMeta{}, topErrorValue(operation, metadata.StatusCode, metadata.Header, response, client.clock.Now())
	}
	requestID := scalarString(root["request_id"])
	if rootCode := scalarString(root["code"]); rootCode != "" && rootCode != "0" {
		return ResponseMeta{}, businessError(operation, rootCode, firstNonEmpty(
			scalarString(root["message"]), scalarString(root["msg"]), "AliExpress returned a nonzero response code",
		), requestID)
	}
	responseRaw := raw
	responseKey := strings.ReplaceAll(method, ".", "_") + "_response"
	if envelope, found := root[responseKey]; found {
		if len(bytes.TrimSpace(envelope)) == 0 || bytes.Equal(bytes.TrimSpace(envelope), []byte("null")) {
			return ResponseMeta{}, platformContractError(operation, "AliExpress method response envelope is empty")
		}
		responseRaw = envelope
	}
	var response struct {
		RequestID  string          `json:"request_id"`
		RespResult json.RawMessage `json:"resp_result"`
	}
	if err := json.Unmarshal(responseRaw, &response); err != nil {
		return ResponseMeta{}, platformContractError(operation, "AliExpress method response envelope is malformed")
	}
	requestID = firstNonEmpty(requestID, response.RequestID, firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"))
	if len(bytes.TrimSpace(response.RespResult)) == 0 || bytes.Equal(bytes.TrimSpace(response.RespResult), []byte("null")) {
		return ResponseMeta{}, platformContractError(operation, "AliExpress response omitted resp_result")
	}
	var result struct {
		Code    json.RawMessage `json:"resp_code"`
		Message string          `json:"resp_msg"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(response.RespResult, &result); err != nil {
		return ResponseMeta{}, platformContractError(operation, "AliExpress resp_result is malformed")
	}
	responseCode := scalarString(result.Code)
	if responseCode != "" && responseCode != "0" && responseCode != "200" {
		return ResponseMeta{}, businessError(operation, responseCode, firstNonEmpty(result.Message, "AliExpress Affiliate method failed"), requestID)
	}
	if len(bytes.TrimSpace(result.Result)) == 0 || bytes.Equal(bytes.TrimSpace(result.Result), []byte("null")) {
		return ResponseMeta{}, platformContractError(operation, "AliExpress response omitted result")
	}
	if output != nil {
		if err := json.Unmarshal(result.Result, output); err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return ResponseMeta{RequestID: boundedMessage(requestID, 256), ResponseCode: responseCode}, nil
}
