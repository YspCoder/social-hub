package jdunion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type businessResult struct {
	Code       ExactValue      `json:"code"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
	TotalCount ExactValue      `json:"totalCount"`
	HasMore    flexibleBool    `json:"hasMore"`
	RequestID  string          `json:"requestId"`
}

type flexibleBool struct {
	value bool
	set   bool
}

func (value *flexibleBool) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	switch string(trimmed) {
	case "true", "\"true\"", "1", "\"1\"":
		value.value, value.set = true, true
		return nil
	case "false", "\"false\"", "0", "\"0\"", "null":
		value.value, value.set = false, true
		return nil
	default:
		return platformContractError("decode_response", "JD hasMore must be a boolean")
	}
}

func (client *Client) doMethod(
	ctx context.Context,
	operation string,
	method string,
	wrapper string,
	input any,
	resultField string,
	options ...socialhub.CallOption,
) (businessResult, ResponseMeta, error) {
	if err := validateCallOptions(operation, options); err != nil {
		return businessResult{}, ResponseMeta{}, err
	}
	paramJSON, err := json.Marshal(map[string]any{wrapper: input})
	if err != nil {
		return businessResult{}, ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	values := make(url.Values)
	values.Set("method", method)
	values.Set("param_json", string(paramJSON))
	payload := values.Encode()
	if len(payload) > maxRequestBytes {
		return businessResult{}, ResponseMeta{}, invalidArgument(operation, "request form exceeds 1 MiB")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, client.gatewayPath, nil, strings.NewReader(payload), options...)
	if err != nil {
		return businessResult{}, ResponseMeta{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return businessResult{}, ResponseMeta{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK || len(raw) == 0 || !json.Valid(raw) {
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD returned an invalid successful response")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD response must be a JSON object")
	}
	if errorRaw, found := root["error_response"]; found && hasJSONValue(errorRaw) {
		response, err := decodeJDError(errorRaw)
		if err != nil {
			return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD returned a malformed error_response")
		}
		return businessResult{}, ResponseMeta{}, jdErrorValue(operation, metadata.StatusCode, metadata.Header, response, client.clock.Now())
	}
	responsePrefix := strings.ReplaceAll(method, ".", "_")
	responseRaw, found := root[responsePrefix+"_responce"]
	if !found {
		responseRaw, found = root[responsePrefix+"_response"]
	}
	if !found {
		if direct, err := decodeJDError(raw); err == nil && direct.code() != "" {
			return businessResult{}, ResponseMeta{}, jdErrorValue(operation, metadata.StatusCode, metadata.Header, direct, client.clock.Now())
		}
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD response omitted the expected method envelope")
	}
	envelopeBytes, err := decodeObjectOrString(responseRaw)
	if err != nil || len(envelopeBytes) == 0 || envelopeBytes[0] != '{' {
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD success envelope must be an object")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(envelopeBytes, &envelope); err != nil {
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD returned a malformed method envelope")
	}
	resultRaw, found := envelope[resultField]
	if !found {
		resultRaw, found = envelope["result"]
	}
	if !found && envelope["data"] != nil && envelope["code"] != nil {
		resultRaw, found = responseRaw, true
	}
	if !found || !hasJSONValue(resultRaw) {
		if response, err := decodeJDError(envelopeBytes); err == nil && response.code() != "" {
			return businessResult{}, ResponseMeta{}, jdErrorValue(operation, metadata.StatusCode, metadata.Header, response, client.clock.Now())
		}
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD method envelope omitted its result")
	}
	resultBytes, err := decodeObjectOrString(resultRaw)
	if err != nil || len(resultBytes) == 0 || resultBytes[0] != '{' {
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD business result must be an object or an encoded object")
	}
	var result businessResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return businessResult{}, ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	code := result.Code.String()
	if code == "" {
		return businessResult{}, ResponseMeta{}, platformContractError(operation, "JD business result omitted code")
	}
	meta := ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		result.RequestID, scalarString(envelope["requestId"]), scalarString(envelope["request_id"]),
		scalarString(root["requestId"]), scalarString(root["request_id"]),
		firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"),
	), 256)}
	if code != "200" {
		return businessResult{}, ResponseMeta{}, jdErrorValue(operation, metadata.StatusCode, metadata.Header, jdErrorResponse{
			Code: result.Code.Bytes(), Message: result.Message, RequestID: meta.RequestID,
		}, client.clock.Now())
	}
	return result, meta, nil
}

func hasJSONValue(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}
