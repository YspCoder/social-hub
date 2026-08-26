package partnerize

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

func (client *Client) doJSON(
	ctx context.Context,
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
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return ResponseMeta{}, invalidArgument(operation, "request JSON exceeds 1 MiB")
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	responseMeta := ResponseMeta{
		RequestID: boundedMessage(redactSensitive(firstNonEmpty(
			firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID,
		)), 256),
		RateLimitLimit: boundedMessage(redactSensitive(
			firstHeader(metadata.Header, "X-RateLimit-Limit"),
		), 64),
		RateLimitRemaining: boundedMessage(redactSensitive(
			firstHeader(metadata.Header, "X-RateLimit-Remaining"),
		), 64),
		RateLimitReset: boundedMessage(redactSensitive(
			firstHeader(metadata.Header, "X-RateLimit-Reset"),
		), 64),
		RateLimitRetryAfter: boundedMessage(redactSensitive(
			firstHeader(metadata.Header, "X-RateLimit-Retry-After", "Retry-After"),
		), 64),
	}
	if err != nil {
		return responseMeta, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return responseMeta, platformContractError(operation, "Partnerize returned an unexpected successful HTTP status", metadata.StatusCode)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return responseMeta, platformContractError(operation, "Partnerize returned an empty or invalid JSON success response", metadata.StatusCode)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return responseMeta, platformContractError(operation, "Partnerize returned a non-JSON success response", metadata.StatusCode)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return responseMeta, withHTTPStatus(
			platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err),
			metadata.StatusCode,
		)
	}
	return responseMeta, nil
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (ResponseMeta, error) {
	return client.doJSON(ctx, operation, http.MethodGet, path, query, nil, output, options...)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) (ResponseMeta, error) {
	return client.doJSON(ctx, operation, http.MethodPost, path, query, input, output, options...)
}
