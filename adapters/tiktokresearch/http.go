package tiktokresearch

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

type responseEnvelope[T any] struct {
	Data  *T             `json:"data"`
	Error *ProviderError `json:"error"`
}

func (client *Client) postJSON(
	ctx context.Context,
	operation string,
	path string,
	fields []string,
	input any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, nil, err
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > maximumRequestBodyBytes {
		return ResponseMeta{}, nil, invalidArgument(operation, "request body is invalid or oversized")
	}
	query := make(url.Values)
	query.Set("fields", strings.Join(fields, ","))
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, query, bytes.NewReader(encoded), options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	meta := responseMeta(metadata.Header, client.clock)
	if err != nil {
		return meta, nil, withOperation(err, operation)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return meta, nil, platformContractError(operation, "TikTok returned an empty, oversized, invalid, or non-object JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "TikTok returned a non-JSON success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), trimmed...), nil
}

func responseMeta(header http.Header, clock socialhub.Clock) ResponseMeta {
	retryAfter := boundedSafeValue(header.Get("Retry-After"), 128)
	return ResponseMeta{
		LogID:      boundedSafeValue(firstHeaderValue(header, "X-Tt-Logid", "X-TikTok-Log-Id", "X-Request-ID"), 512),
		RetryAfter: retryAfter, RetryAfterDuration: parseRetryAfter(retryAfter, clock.Now()),
	}
}

func firstHeaderValue(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
