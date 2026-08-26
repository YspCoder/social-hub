package googleplaces

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) doJSON(
	ctx context.Context,
	operation, method, path string,
	query url.Values,
	input, output any,
	fieldMask string,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
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
	if fieldMask != "" {
		request.Header.Set("X-Goog-FieldMask", fieldMask)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	meta := responseMeta(metadata.Header, client.clock.Now(), client.apiKey)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) || trimmed[0] != '{' {
		return meta, platformContractError(operation, "Google returned an empty, invalid, or non-object JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, platformContractError(operation, "Google returned a non-JSON success response")
	}
	sanitized := sanitizeProviderBody(trimmed, client.apiKey)
	var envelope ProviderErrorEnvelope
	if err := json.Unmarshal(sanitized, &envelope); err != nil {
		return meta, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if envelope.Error != nil {
		return meta, newProviderAPIError(operation, metadata.StatusCode, meta, envelope, sanitized)
	}
	if err := json.Unmarshal(sanitized, output); err != nil {
		return meta, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, nil
}

func responseMeta(header http.Header, now time.Time, secret string) ResponseMeta {
	retryAfterHeader := cleanHeader(header.Get("Retry-After"), 128, secret)
	return ResponseMeta{
		RequestID:        cleanHeader(firstHeaderValue(header, "X-Goog-Request-Id", "X-Google-Request-Id", "X-Request-Id"), 256, secret),
		TraceContext:     cleanHeader(firstHeaderValue(header, "X-Cloud-Trace-Context", "X-Google-Gfe-Request-Trace"), 512, secret),
		RetryAfterHeader: retryAfterHeader,
		RetryAfter:       parseRetryAfter(retryAfterHeader, now),
		QuotaHeaders:     quotaHeaders(header, secret),
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

func quotaHeaders(header http.Header, secret string) map[string]string {
	result := make(map[string]string)
	for name, values := range header {
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, "x-goog-quota-") && !strings.HasPrefix(lower, "x-ratelimit-") &&
			!strings.HasPrefix(lower, "ratelimit-") {
			continue
		}
		if len(result) >= 32 {
			break
		}
		result[http.CanonicalHeaderKey(name)] = cleanHeader(strings.Join(values, ","), 1024, secret)
	}
	if len(result) == 0 {
		return nil
	}
	return result
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
