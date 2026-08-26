package impactpartner

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	method string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	request, err := client.api.NewRequest(ctx, method, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	request.Header.Set("IR-Version", apiVersion)
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	responseMeta := ResponseMeta{
		RequestID:              boundedMessage(firstNonEmpty(firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID), 256),
		RateLimitLimitHour:     boundedMessage(firstHeader(metadata.Header, "X-RateLimit-Limit-hour"), 64),
		RateLimitRemainingHour: boundedMessage(firstHeader(metadata.Header, "X-RateLimit-Remaining-hour"), 64),
		RateLimitReset:         boundedMessage(firstHeader(metadata.Header, "RateLimit-Reset"), 64),
	}
	if err != nil {
		return responseMeta, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return responseMeta, platformContractError(operation, "impact.com returned an unexpected successful HTTP status", metadata.StatusCode)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return responseMeta, platformContractError(operation, "impact.com returned an empty or invalid JSON success response", metadata.StatusCode)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return responseMeta, platformContractError(operation, "impact.com returned a non-JSON success response", metadata.StatusCode)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return responseMeta, withHTTPStatus(
			platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err),
			metadata.StatusCode,
		)
	}
	return responseMeta, nil
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (ResponseMeta, error) {
	return client.doJSON(ctx, operation, http.MethodGet, path, query, output, options...)
}

func (client *Client) postWithoutBody(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (ResponseMeta, error) {
	return client.doJSON(ctx, operation, http.MethodPost, path, query, output, options...)
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
