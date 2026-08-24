package openverse

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
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	meta := responseMeta(metadata.Header)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return meta, nil, platformContractError(operation, "Openverse returned an empty or invalid JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, append(json.RawMessage(nil), raw...), platformContractError(operation, "Openverse returned a non-JSON success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, append(json.RawMessage(nil), raw...), platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), raw...), nil
}

func responseMeta(header http.Header) ResponseMeta {
	rateLimits := make(map[string]RateLimit)
	for name, values := range header {
		if len(values) == 0 {
			continue
		}
		lowerName := strings.ToLower(name)
		const limitPrefix = "x-ratelimit-limit-"
		const availablePrefix = "x-ratelimit-available-"
		switch {
		case strings.HasPrefix(lowerName, limitPrefix):
			scope := strings.TrimPrefix(lowerName, limitPrefix)
			entry := rateLimits[scope]
			entry.Limit = boundedMessage(values[0], 64)
			rateLimits[scope] = entry
		case strings.HasPrefix(lowerName, availablePrefix):
			scope := strings.TrimPrefix(lowerName, availablePrefix)
			entry := rateLimits[scope]
			entry.Available = boundedMessage(values[0], 64)
			rateLimits[scope] = entry
		}
	}
	if len(rateLimits) == 0 {
		rateLimits = nil
	}
	return ResponseMeta{
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 256),
		RateLimits: rateLimits,
	}
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
