package vipunion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type vipEnvelope struct {
	ReturnCode    json.RawMessage `json:"returnCode"`
	ReturnMessage string          `json:"returnMessage"`
	Result        json.RawMessage `json:"result"`
}

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	service string,
	method string,
	requestID string,
	input any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(payload) == 0 || len(payload) > maxRequestBytes {
		return ResponseMeta{}, invalidArgument(operation, "request body is empty or exceeds 1 MiB")
	}
	query := url.Values{
		"service": {service},
		"method":  {method},
		"version": {apiVersion},
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/", query, bytes.NewReader(payload), options...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation, requestID)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation, requestID)
	}
	if metadata.StatusCode != http.StatusOK || len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, platformContractError(operation, "Vipshop returned an invalid successful response")
	}
	var envelope vipEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	returnCode := scalarString(envelope.ReturnCode)
	if returnCode == "" {
		return ResponseMeta{}, platformContractError(operation, "Vipshop response omitted returnCode")
	}
	if returnCode != "0" {
		return ResponseMeta{}, vipErrorValue(operation, metadata.StatusCode, metadata.Header, vipErrorResponse{
			ReturnCode: envelope.ReturnCode, ReturnMessage: envelope.ReturnMessage,
		}, requestID, client.clock.Now())
	}
	if output != nil {
		trimmed := bytes.TrimSpace(envelope.Result)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
			return ResponseMeta{}, platformContractError(operation, "Vipshop response omitted result")
		}
		if err := json.Unmarshal(trimmed, output); err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		requestID, firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"),
	), 256)}, nil
}
