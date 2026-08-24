package steam

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(
	ctx context.Context,
	api *transport.Client,
	operation string,
	path string,
	query url.Values,
	webAPIKey string,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, nil, err
	}
	request, err := api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	var raw json.RawMessage
	metadata, err := api.DoWithMetadata(request, &raw)
	meta := responseMeta(metadata.StatusCode, metadata.Header, client.clock, webAPIKey)
	if err != nil {
		return meta, nil, withOperation(err, operation)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != '{' {
		return meta, nil, platformContractError(operation, "Steam returned an empty, oversized, invalid, or unexpected JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "Steam returned a non-JSON success response")
	}
	if webAPIKey != "" && bytes.Contains(trimmed, []byte(webAPIKey)) {
		return meta, nil, platformContractError(operation, "Steam returned credential material in a success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), trimmed...), nil
}

func responseMeta(status int, header http.Header, clock socialhub.Clock, webAPIKey string) ResponseMeta {
	retryAfter := boundedMessage(redactExact(header.Get("Retry-After"), webAPIKey), 128)
	return ResponseMeta{
		StatusCode:         status,
		RetryAfter:         retryAfter,
		RetryAfterDuration: parseRetryAfter(retryAfter, clock.Now()),
	}
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
