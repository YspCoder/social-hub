package itunessearch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes int64 = 8 << 20

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	options ...socialhub.CallOption,
) (json.RawMessage, ResponseMeta, error) {
	if ctx == nil {
		return nil, ResponseMeta{}, invalidArgument(operation, "context must not be nil")
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	httpClient, clock, err := client.dependencies(operation)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	requestURL := baseURL + path
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, ResponseMeta{}, transportError(operation, err)
	}
	request.Header.Set("Accept", "application/json, text/javascript")
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, ResponseMeta{}, transportError(operation, err)
	}
	defer response.Body.Close()
	meta := responseMeta(response.Header, clock)
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProviderObjectBytes+1))
	if err != nil {
		return nil, meta, transportError(operation, err)
	}
	if int64(len(body)) > maxProviderObjectBytes {
		return nil, meta, platformContractError(operation, "Apple response exceeded the 8 MiB safety limit")
	}
	trimmed := bytes.TrimSpace(body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, meta, decodeHTTPError(operation, response.StatusCode, response.Header, trimmed, clock)
	}
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return nil, meta, platformContractError(operation, "Apple returned an empty, invalid, or non-object JSON success response")
	}
	if !validAppleJSONContentType(response.Header.Get("Content-Type")) {
		return nil, meta, platformContractError(operation, "Apple returned an unexpected success Content-Type")
	}
	return append(json.RawMessage(nil), trimmed...), meta, nil
}

func validAppleJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	normalized := strings.ToLower(mediaType)
	return normalized == "application/json" || strings.HasSuffix(normalized, "+json") ||
		normalized == "text/javascript"
}

func decodeCatalogResponse(operation string, raw json.RawMessage, meta ResponseMeta, maximum int) (*CatalogResponse, error) {
	var envelope struct {
		ResultCount *int            `json:"resultCount"`
		Results     json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.ResultCount == nil {
		return nil, platformContractError(operation, "Apple response omitted a valid resultCount")
	}
	resultJSON := bytes.TrimSpace(envelope.Results)
	if len(resultJSON) == 0 || resultJSON[0] != '[' {
		return nil, platformContractError(operation, "Apple response omitted a results array")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(resultJSON, &items); err != nil {
		return nil, platformContractError(operation, "Apple returned an invalid results array")
	}
	if *envelope.ResultCount < 0 || *envelope.ResultCount != len(items) || maximum > 0 && len(items) > maximum {
		return nil, platformContractError(operation, "Apple returned inconsistent resultCount or result limits")
	}
	results := make([]Result, len(items))
	for index, item := range items {
		trimmed := bytes.TrimSpace(item)
		if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &results[index]) != nil {
			return nil, platformContractError(operation, "Apple returned a non-object or invalid catalog result")
		}
	}
	return &CatalogResponse{ResultCount: *envelope.ResultCount, Results: results, Meta: meta}, nil
}
