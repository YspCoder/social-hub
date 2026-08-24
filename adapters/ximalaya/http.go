package ximalaya

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
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	meta := responseMeta(metadata.StatusCode, metadata.Header, client.clock, client.redactions)
	if err != nil {
		return meta, nil, withOperation(err, operation)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) {
		return meta, nil, platformContractError(operation, "Ximalaya returned an empty, oversized, or invalid JSON success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, nil, platformContractError(operation, "Ximalaya returned a non-JSON success response")
	}
	if providerErr, found := providerErrorFromBody(metadata.StatusCode, metadata.Header, trimmed, client.clock, client.redactions); found {
		return meta, nil, withOperation(providerErr, operation)
	}
	if trimmed[0] != expected {
		return meta, nil, platformContractError(operation, "Ximalaya returned an unexpected JSON success shape")
	}
	for _, secret := range client.secrets {
		if secret != "" && bytes.Contains(trimmed, []byte(secret)) {
			return meta, nil, platformContractError(operation, "Ximalaya returned credential material in a success response")
		}
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), trimmed...), nil
}

func responseMeta(status int, header http.Header, clock socialhub.Clock, redactions []string) ResponseMeta {
	retryAfter := boundedMessage(redactExact(header.Get("Retry-After"), redactions...), 128)
	return ResponseMeta{
		StatusCode: status, RetryAfter: retryAfter,
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
