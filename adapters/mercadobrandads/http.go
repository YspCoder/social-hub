package mercadobrandads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	version string,
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
	if version != "" {
		request.Header.Set("Api-Version", version)
	}
	request.Header.Set("Accept-Encoding", "identity")
	var raw json.RawMessage
	metadata, err := api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	meta := ResponseMeta{RequestID: safeRequestID(metadata.Header.Get("X-Request-Id"), nil)}
	if metadata.StatusCode != http.StatusOK {
		return meta, platformContractError(operation, "Mercado Libre returned an unexpected successful HTTP status")
	}
	if !validJSONMediaType(metadata.Header.Get("Content-Type")) {
		return meta, platformContractError(operation, "Mercado Libre success response was not JSON")
	}
	if output == nil || len(raw) == 0 || !json.Valid(raw) {
		return meta, platformContractError(operation, "Mercado Libre success response omitted valid JSON data")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return meta, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, nil
}
