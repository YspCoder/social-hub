package gitlab

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
	expected byte,
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
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != expected {
		return meta, nil, platformContractError(operation, "GitLab returned an empty, oversized, invalid, or unexpected JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "GitLab returned a non-JSON success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), trimmed...), nil
}

func responseMeta(header http.Header, clock socialhub.Clock) ResponseMeta {
	now := clock.Now()
	meta := ResponseMeta{
		RequestID:          boundedMessage(header.Get("X-Request-Id"), 256),
		GitLabMeta:         boundedMessage(header.Get("X-GitLab-Meta"), 4096),
		RateLimitLimit:     boundedMessage(header.Get("RateLimit-Limit"), 64),
		RateLimitName:      boundedMessage(header.Get("RateLimit-Name"), 256),
		RateLimitObserved:  boundedMessage(header.Get("RateLimit-Observed"), 64),
		RateLimitRemaining: boundedMessage(header.Get("RateLimit-Remaining"), 64),
		RateLimitReset:     boundedMessage(header.Get("RateLimit-Reset"), 64),
		RateLimitResetTime: boundedMessage(header.Get("RateLimit-ResetTime"), 128),
		RetryAfter:         boundedMessage(header.Get("Retry-After"), 128),
		ETag:               boundedMessage(header.Get("ETag"), 1024),
		Link:               boundedMessage(header.Get("Link"), 16_384),
		Page:               boundedMessage(header.Get("X-Page"), 64),
		PerPage:            boundedMessage(header.Get("X-Per-Page"), 64),
		NextPage:           boundedMessage(header.Get("X-Next-Page"), 64),
		PreviousPage:       boundedMessage(header.Get("X-Prev-Page"), 64),
		Total:              boundedMessage(header.Get("X-Total"), 64),
		TotalPages:         boundedMessage(header.Get("X-Total-Pages"), 64),
	}
	if seconds, err := strconv.ParseInt(strings.TrimSpace(meta.RateLimitReset), 10, 64); err == nil && seconds > 0 {
		at := time.Unix(seconds, 0).UTC()
		meta.RateLimitResetAt = &at
		meta.RateLimitResetAfter = boundedDelay(at.Sub(now))
	}
	meta.RetryAfterDuration = parseRetryAfter(meta.RetryAfter, now)
	return meta
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
