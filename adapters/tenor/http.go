package tenor

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
	meta := responseMeta(metadata.Header)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return meta, platformContractError(operation, "Tenor returned an empty or invalid JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, platformContractError(operation, "Tenor returned a non-JSON success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, nil
}

func responseMeta(header http.Header) ResponseMeta {
	return ResponseMeta{
		CacheControl: boundedMessage(header.Get("Cache-Control"), 4096),
		ETag:         boundedMessage(header.Get("ETag"), 1024),
		RequestID:    boundedMessage(firstNonEmpty(header.Get("X-Google-Request-ID"), header.Get("X-Request-ID")), 256),
	}
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
