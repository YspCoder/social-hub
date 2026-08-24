package amazoncreators

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) postJSON(
	ctx context.Context,
	operation string,
	path string,
	input any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	body, err := json.Marshal(input)
	if err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(body), options...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-marketplace", client.marketplace)
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, platformContractError(operation, "Amazon returned an empty or invalid successful response")
	}
	contentType := strings.TrimSpace(strings.Split(metadata.Header.Get("Content-Type"), ";")[0])
	if contentType != "" && contentType != "application/json" {
		return ResponseMeta{}, platformContractError(operation, "Amazon returned a non-JSON successful response")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		firstHeader(metadata.Header, "x-amzn-RequestId", "x-amzn-requestid", "X-Request-ID"),
		callOptions.RequestID,
	), 256)}, nil
}
