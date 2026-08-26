package sovrncommerce

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const maximumRequestBytes = 1 << 20

func (client *Client) doJSON(
	ctx context.Context,
	api *transport.Client,
	operation string,
	method string,
	path string,
	query url.Values,
	input any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	if len(path)+len(query.Encode()) > maximumRequestBytes {
		return ResponseMeta{}, invalidArgument(operation, "encoded request URL exceeds the adapter's 1 MiB safety limit")
	}
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maximumRequestBytes {
			return ResponseMeta{}, invalidArgument(operation, "encoded request body exceeds the adapter's 1 MiB safety limit")
		}
		body = bytes.NewReader(encoded)
	}
	var requestOptions []socialhub.CallOption
	if callOptions.Timeout > 0 {
		requestOptions = []socialhub.CallOption{socialhub.WithCallTimeout(callOptions.Timeout)}
	}
	request, err := api.NewRequest(ctx, method, path, query, body, requestOptions...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	var raw json.RawMessage
	metadata, err := api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return ResponseMeta{}, platformContractError(operation, "Sovrn returned an undocumented successful HTTP status", metadata.StatusCode)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return ResponseMeta{}, platformContractError(operation, "Sovrn returned an empty, oversized, or invalid JSON object", metadata.StatusCode)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return ResponseMeta{}, platformContractError(operation, "Sovrn returned a non-JSON successful response", metadata.StatusCode)
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return ResponseMeta{}, platformContractError(operation, "Sovrn returned JSON that does not match the documented response", metadata.StatusCode)
	}
	secrets := client.redactionSecrets
	return ResponseMeta{
		RequestID:    firstSafeHeader(metadata.Header, 256, secrets, "X-Request-ID", "X-Correlation-ID"),
		ETag:         firstSafeHeader(metadata.Header, 1024, secrets, "ETag"),
		LastModified: firstSafeHeader(metadata.Header, 128, secrets, "Last-Modified"),
		RetryAfter:   firstSafeHeader(metadata.Header, 128, secrets, "Retry-After"),
	}, nil
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}

func (client *Client) getJSON(ctx context.Context, api *transport.Client, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (ResponseMeta, error) {
	return client.doJSON(ctx, api, operation, http.MethodGet, path, query, nil, output, options...)
}

func (client *Client) postJSON(ctx context.Context, api *transport.Client, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) (ResponseMeta, error) {
	return client.doJSON(ctx, api, operation, http.MethodPost, path, query, input, output, options...)
}
