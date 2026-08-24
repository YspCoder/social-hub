package blogger

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
) (ResponseMeta, json.RawMessage, error) {
	if err := client.requireReadScope(operation); err != nil {
		return ResponseMeta{}, nil, err
	}
	if ctx == nil {
		return ResponseMeta{}, nil, invalidArgument(operation, "context must not be nil")
	}
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, nil, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")

	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	meta := responseMeta(metadata.Header, client.clock, client.accessToken)
	if err != nil {
		return meta, nil, withOperation(err, operation)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return meta, nil, platformContractError(operation, "Blogger returned an empty, oversized, invalid, or non-object JSON success response")
	}

	var envelope map[string]json.RawMessage
	if json.Unmarshal(trimmed, &envelope) != nil {
		return meta, nil, platformContractError(operation, "Blogger returned an invalid JSON success object")
	}
	if encoded, exists := envelope["error"]; exists && !bytes.Equal(bytes.TrimSpace(encoded), []byte("null")) {
		decode := newHTTPErrorDecoder(client.clock, client.accessToken)
		return meta, nil, withOperation(decode(metadata.StatusCode, metadata.Header, trimmed), operation)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "Blogger returned a non-JSON success response")
	}

	sanitized, ok := sanitizeProviderJSON(trimmed, client.accessToken, maxProviderObjectBytes)
	if !ok {
		return meta, nil, platformContractError(operation, "Blogger success JSON could not be safely retained")
	}
	if err := json.Unmarshal(sanitized, output); err != nil {
		return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, sanitized, nil
}

func responseMeta(header http.Header, clock socialhub.Clock, accessToken string) ResponseMeta {
	retryAfterHeader := boundedMessage(redactText(header.Get("Retry-After"), accessToken), 128)
	return ResponseMeta{
		RequestID: boundedMessage(redactText(firstHeaderValue(header,
			"X-Goog-Request-ID", "X-Google-Request-ID", "X-Request-ID"), accessToken), 512),
		TraceContext: boundedMessage(redactText(firstHeaderValue(header,
			"Traceparent", "X-Cloud-Trace-Context", "X-Google-GFE-Request-Trace"), accessToken), 1024),
		ETag:             boundedMessage(redactText(header.Get("ETag"), accessToken), 1024),
		RetryAfterHeader: retryAfterHeader,
		RetryAfter:       parseRetryAfter(retryAfterHeader, clock.Now()),
		QuotaHeaders:     dynamicQuotaHeaders(header, accessToken),
	}
}

func dynamicQuotaHeaders(header http.Header, accessToken string) map[string]string {
	names := make([]string, 0, len(header))
	for name := range header {
		normalized := strings.ToLower(name)
		if strings.Contains(normalized, "quota") || strings.HasPrefix(normalized, "x-ratelimit-") ||
			strings.HasPrefix(normalized, "ratelimit-") || strings.HasPrefix(normalized, "rate-limit-") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	if len(names) > 64 {
		names = names[:64]
	}
	result := make(map[string]string, len(names))
	for _, name := range names {
		value := strings.Join(header.Values(name), ", ")
		result[boundedMessage(name, 256)] = boundedMessage(redactText(value, accessToken), 4096)
	}
	return result
}

func firstHeaderValue(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((48*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay <= 0 || delay > 48*time.Hour {
		return 0
	}
	return delay
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
