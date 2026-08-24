package pixabay

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
	meta := responseMeta(metadata.Header, client.apiKey)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) || trimmed[0] != '{' {
		return meta, nil, platformContractError(operation, "Pixabay returned an empty, invalid, or non-object JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, append(json.RawMessage(nil), raw...), platformContractError(operation, "Pixabay returned a non-JSON success response")
	}
	if containsCredential(trimmed, client.apiKey) {
		return meta, nil, platformContractError(operation, "Pixabay returned the configured API key in a success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, append(json.RawMessage(nil), raw...), platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), raw...), nil
}

func responseMeta(header http.Header, apiKey string) ResponseMeta {
	reset := cleanHeader(header.Get("X-RateLimit-Reset"), 128, apiKey)
	return ResponseMeta{
		RequestID:           cleanHeader(firstNonEmpty(header.Get("X-Request-ID"), header.Get("CF-Ray")), 256, apiKey),
		CacheControl:        cleanHeader(header.Get("Cache-Control"), 4096, apiKey),
		RateLimitLimit:      cleanHeader(header.Get("X-RateLimit-Limit"), 128, apiKey),
		RateLimitRemaining:  cleanHeader(header.Get("X-RateLimit-Remaining"), 128, apiKey),
		RateLimitReset:      reset,
		RateLimitResetAfter: parseResetSeconds(reset),
		RequiredCacheTTL:    RequiredCacheTTL,
	}
}

func cleanHeader(value string, maximum int, apiKey string) string {
	return boundedMessage(redactErrorText(value, apiKey), maximum)
}

func containsCredential(body []byte, apiKey string) bool {
	if apiKey == "" {
		return false
	}
	return bytes.Contains(body, []byte(apiKey)) || bytes.Contains(body, []byte(url.QueryEscape(apiKey)))
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
