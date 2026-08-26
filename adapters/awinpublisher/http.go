package awinpublisher

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

const maxRequestBytes = 1 << 20

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	method string,
	path string,
	query url.Values,
	input any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, nil, err
	}
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return ResponseMeta{}, nil, invalidArgument(operation, "request JSON exceeds 1 MiB")
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, nil, platformContractError(operation, "Awin returned an empty or invalid successful response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return ResponseMeta{}, nil, platformContractError(operation, "Awin returned a non-JSON successful response")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return ResponseMeta{
		RequestID: boundedMessage(firstNonEmpty(
			firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID,
		), 256),
	}, append(json.RawMessage(nil), raw...), nil
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	mediaType = strings.ToLower(mediaType)
	return err == nil && (mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"))
}

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	return client.doJSON(ctx, operation, http.MethodGet, path, query, nil, output, options...)
}

func (client *Client) postJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	input any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	return client.doJSON(ctx, operation, http.MethodPost, path, query, input, output, options...)
}
