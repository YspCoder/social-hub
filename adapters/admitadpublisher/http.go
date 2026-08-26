package admitadpublisher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	requiredScope string,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, withOperationAndScope(err, operation, requiredScope)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperationAndScope(err, operation, requiredScope)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, platformContractError(operation, "Admitad returned an empty or invalid successful response")
	}
	contentType := strings.TrimSpace(strings.Split(metadata.Header.Get("Content-Type"), ";")[0])
	if contentType != "" && contentType != "application/json" && !strings.HasSuffix(contentType, "+json") {
		return ResponseMeta{}, platformContractError(operation, "Admitad returned a non-JSON successful response")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID,
	), 256)}, nil
}
