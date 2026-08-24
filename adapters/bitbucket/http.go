package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
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
	if err != nil {
		return responseMeta(metadata.Header, client.clock), nil, withOperation(err, operation)
	}
	meta := responseMeta(metadata.Header, client.clock)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != '{' {
		return meta, nil, platformContractError(operation, "Bitbucket returned an empty, oversized, invalid, or non-object JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "Bitbucket returned a non-JSON success response")
	}
	if output != nil {
		if err := json.Unmarshal(trimmed, output); err != nil {
			return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return meta, append(json.RawMessage(nil), trimmed...), nil
}

func responseMeta(header http.Header, clock socialhub.Clock) ResponseMeta {
	meta := ResponseMeta{
		RequestID:          boundedMessage(firstHeaderValue(header, "Atl-Traceid", "X-Request-Id", "X-Correlation-Id"), 256),
		RequestCount:       boundedMessage(header.Get("X-Request-Count"), 64),
		RateLimitLimit:     boundedMessage(header.Get("X-RateLimit-Limit"), 64),
		RateLimitResource:  boundedMessage(header.Get("X-RateLimit-Resource"), 128),
		RateLimitNearLimit: boundedMessage(header.Get("X-RateLimit-NearLimit"), 64),
		RetryAfter:         boundedMessage(header.Get("Retry-After"), 128),
		ETag:               boundedMessage(header.Get("ETag"), 1024),
		Deprecation:        boundedMessage(header.Get("Deprecation"), 128),
		Sunset:             boundedMessage(header.Get("Sunset"), 128),
		Warning:            boundedMessage(header.Get("Warning"), 1024),
	}
	meta.RetryAfterDuration = parseRetryAfter(meta.RetryAfter, clock.Now())
	return meta
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
