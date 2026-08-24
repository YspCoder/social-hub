package github

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
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return responseMeta(metadata.Header, client.clock), nil, withOperation(err, operation)
	}
	meta := responseMeta(metadata.Header, client.clock)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != expected {
		return meta, nil, platformContractError(operation, "GitHub returned an empty, oversized, invalid, or unexpected JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "GitHub returned a non-JSON success response")
	}
	if meta.APIVersionSelected != "" && meta.APIVersionSelected != apiVersion {
		return meta, nil, platformContractError(operation, "GitHub selected a different REST API version")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), trimmed...), nil
}

func responseMeta(header http.Header, clock socialhub.Clock) ResponseMeta {
	meta := ResponseMeta{
		RequestID:           boundedMessage(header.Get("X-GitHub-Request-Id"), 256),
		APIVersionSelected:  boundedMessage(header.Get("X-GitHub-Api-Version-Selected"), 64),
		MediaType:           boundedMessage(header.Get("X-GitHub-Media-Type"), 256),
		RateLimitLimit:      boundedMessage(header.Get("X-RateLimit-Limit"), 64),
		RateLimitRemaining:  boundedMessage(header.Get("X-RateLimit-Remaining"), 64),
		RateLimitUsed:       boundedMessage(header.Get("X-RateLimit-Used"), 64),
		RateLimitReset:      boundedMessage(header.Get("X-RateLimit-Reset"), 64),
		RateLimitResource:   boundedMessage(header.Get("X-RateLimit-Resource"), 128),
		RetryAfter:          boundedMessage(header.Get("Retry-After"), 128),
		OAuthScopes:         splitHeaderList(header.Get("X-OAuth-Scopes")),
		AcceptedOAuthScopes: splitHeaderList(header.Get("X-Accepted-OAuth-Scopes")),
		SSO:                 boundedMessage(header.Get("X-GitHub-SSO"), 4096),
		ETag:                boundedMessage(header.Get("ETag"), 1024),
		Link:                boundedMessage(header.Get("Link"), 16_384),
		Deprecation:         boundedMessage(header.Get("Deprecation"), 128),
		Sunset:              boundedMessage(header.Get("Sunset"), 128),
		Warning:             boundedMessage(header.Get("Warning"), 1024),
	}
	if seconds, err := strconv.ParseInt(strings.TrimSpace(meta.RateLimitReset), 10, 64); err == nil && seconds > 0 {
		at := time.Unix(seconds, 0).UTC()
		meta.RateLimitResetAt = &at
		meta.RateLimitResetAfter = boundedDelay(at.Sub(clock.Now()))
	}
	meta.RetryAfterDuration = parseRetryAfter(meta.RetryAfter, clock.Now())
	return meta
}

func splitHeaderList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, boundedMessage(part, 256))
		}
	}
	return result
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
