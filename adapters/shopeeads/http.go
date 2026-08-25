package shopeeads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type responseEnvelope struct {
	Error     string          `json:"error"`
	Message   string          `json:"message"`
	Warning   string          `json:"warning"`
	RequestID string          `json:"request_id"`
	Response  json.RawMessage `json:"response"`
}

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	if ctx == nil {
		return ResponseMeta{}, invalidArgument(operation, "context is required")
	}
	api, err := client.apiClient(operation)
	if err != nil {
		return ResponseMeta{}, err
	}
	resolved, err := resolveCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	request, err := api.NewRequest(ctx, http.MethodGet, path, query, nil, resolvedCallOption(resolved))
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	var raw json.RawMessage
	metadata, err := api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return ResponseMeta{}, platformContractError(operation, "Shopee returned an unexpected successful HTTP status")
	}
	if !validJSONMediaType(metadata.Header.Get("Content-Type")) {
		return ResponseMeta{}, platformContractError(operation, "Shopee success response was not JSON")
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, platformContractError(operation, "Shopee returned invalid JSON")
	}
	var envelope responseEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	requestID := safeRequestID(firstNonEmpty(envelope.RequestID, metadata.Header.Get("X-Shopee-Request-ID")), nil)
	if envelope.Error != "" {
		return ResponseMeta{}, apiErrorValue(
			operation, metadata.StatusCode, metadata.Header,
			envelope.Error, envelope.Message, requestID, client.clock.Now(),
		)
	}
	if output == nil || len(envelope.Response) == 0 || strings.TrimSpace(string(envelope.Response)) == "null" {
		return ResponseMeta{}, platformContractError(operation, "Shopee success response omitted response data")
	}
	if err := json.Unmarshal(envelope.Response, output); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return ResponseMeta{RequestID: requestID, Warning: boundedText(envelope.Warning, 1024)}, nil
}
