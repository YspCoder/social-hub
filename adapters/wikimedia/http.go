package wikimedia

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

const maxResponseBytes int64 = 8 << 20

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	requestURL := client.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", client.userAgent)
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxResponseBytes {
		return ResponseMeta{}, nil, platformContractError(operation, "Wikimedia response exceeded 8 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ResponseMeta{}, nil, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, client.clock.Now()), operation)
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return ResponseMeta{}, nil, platformContractError(operation, "Wikimedia returned a non-JSON success response")
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return ResponseMeta{}, nil, platformContractError(operation, "Wikimedia returned an empty or invalid JSON success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	metadata := ResponseMeta{
		RequestID:    firstNonEmpty(response.Header.Get("X-Request-ID"), callOptions.RequestID),
		CacheControl: boundedMessage(response.Header.Get("Cache-Control"), 1024),
	}
	return metadata, append(json.RawMessage(nil), trimmed...), nil
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
