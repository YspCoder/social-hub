package tripadvisor

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	meta := responseMeta(metadata.Header, client.clock.Now(), client.apiKey)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) || trimmed[0] != '{' {
		return meta, platformContractError(operation, "Tripadvisor returned an empty, invalid, or non-object JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, platformContractError(operation, "Tripadvisor returned a non-JSON success response")
	}
	sanitized := sanitizeProviderBody(trimmed, client.apiKey)
	var envelope struct {
		Error *ProviderError `json:"error"`
	}
	if err := json.Unmarshal(sanitized, &envelope); err != nil {
		return meta, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if envelope.Error != nil {
		return meta, newProviderAPIError(operation, metadata.StatusCode, meta, *envelope.Error, sanitized)
	}
	if err := json.Unmarshal(sanitized, output); err != nil {
		return meta, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, nil
}

func responseMeta(header http.Header, now time.Time, secret string) ResponseMeta {
	retryAfterHeader := cleanHeader(header.Get("Retry-After"), 128, secret)
	return ResponseMeta{
		RequestID:          cleanHeader(header.Get("X-Amzn-Requestid"), 256, secret),
		APIGatewayID:       cleanHeader(header.Get("X-Amz-Apigw-Id"), 256, secret),
		RetryAfterHeader:   retryAfterHeader,
		RetryAfter:         parseRetryAfter(retryAfterHeader, now),
		RateLimitLimit:     cleanHeader(firstHeaderValue(header, "X-RateLimit-Limit", "X-Rate-Limit-Limit", "RateLimit-Limit"), 128, secret),
		RateLimitRemaining: cleanHeader(firstHeaderValue(header, "X-RateLimit-Remaining", "X-Rate-Limit-Remaining", "RateLimit-Remaining"), 128, secret),
		RateLimitReset:     cleanHeader(firstHeaderValue(header, "X-RateLimit-Reset", "X-Rate-Limit-Reset", "RateLimit-Reset"), 128, secret),
	}
}

func firstHeaderValue(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func cleanHeader(value string, maximum int, secret string) string {
	return boundedMessage(redactSensitive(redactExact(value, secret)), maximum)
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
