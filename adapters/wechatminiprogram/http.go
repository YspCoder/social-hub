package wechatminiprogram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxResponseBytes int64 = 1 << 20

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	method string,
	path string,
	query url.Values,
	input any,
	output any,
	options ...socialhub.CallOption,
) error {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBodyBytes {
			return invalidArgument(operation, "request body exceeds the adapter's 64 KiB safety limit")
		}
		body = bytes.NewReader(encoded)
	}
	requestURL := baseURL + "/" + strings.TrimLeft(path, "/")
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(responseBody)) > maxResponseBytes {
		return platformContractError(operation, "WeChat response exceeded the 1 MiB safety limit")
	}
	trimmed := bytes.TrimSpace(responseBody)
	retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), client.clock.Now())
	var provider apiResponse
	providerDecoded := len(trimmed) != 0 && trimmed[0] == '{' && json.Unmarshal(trimmed, &provider) == nil
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if providerDecoded && provider.ErrCode != 0 {
			return businessError(operation, response.StatusCode, provider, retryAfter)
		}
		return httpError(operation, response.StatusCode, response.Header, retryAfter)
	}
	if !providerDecoded {
		return platformContractError(operation, "WeChat returned an empty or invalid JSON response")
	}
	if provider.ErrCode != 0 {
		return businessError(operation, response.StatusCode, provider, retryAfter)
	}
	if output != nil {
		if err := json.Unmarshal(trimmed, output); err != nil {
			return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return nil
}
