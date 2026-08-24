package lineads

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, forwardCallOptions(callOptions)...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, platformContractError(operation, "LINE Ads returned an empty or invalid successful response")
	}
	contentType, _, contentTypeErr := mime.ParseMediaType(metadata.Header.Get("Content-Type"))
	if contentTypeErr != nil || contentType != "application/json" {
		return ResponseMeta{}, platformContractError(operation, "LINE Ads returned a non-JSON successful response")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return ResponseMeta{
		RequestQuotaLimit: safeHeaderValue(metadata.Header.Get("X-Request-Quota-Limit"), 64),
		RequestQuotaUsed:  safeHeaderValue(metadata.Header.Get("X-Request-Quota-Used"), 64),
	}, nil
}
