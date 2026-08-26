package skimlinks

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

type ResponseMeta struct {
	StatusCode int
	RequestID  string
}

func (client *Client) getJSON(
	ctx context.Context,
	api *transport.Client,
	operation string,
	path string,
	query url.Values,
	expectedStatus int,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	_, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, nil, err
	}
	request, err := api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	var raw json.RawMessage
	metadata, err := api.DoWithMetadata(request, &raw)
	responseMeta := client.responseMeta(metadata.StatusCode, metadata.Header)
	rawCopy := append(json.RawMessage(nil), raw...)
	if err != nil {
		if providerRaw := rawFromAPIError(err); len(providerRaw) > 0 {
			rawCopy = providerRaw
		}
		return responseMeta, client.errorRaw(rawCopy), withOperation(err, operation)
	}
	if metadata.StatusCode != expectedStatus {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Skimlinks returned an unexpected successful HTTP status", metadata.StatusCode,
		)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Skimlinks returned an empty or invalid JSON success response", metadata.StatusCode,
		)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Skimlinks returned a non-JSON success response", metadata.StatusCode,
		)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return responseMeta, client.errorRaw(rawCopy), withHTTPStatus(
			platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err), metadata.StatusCode,
		)
	}
	return responseMeta, rawCopy, nil
}

func (client *Client) responseMeta(status int, header http.Header) ResponseMeta {
	return ResponseMeta{
		StatusCode: status,
		RequestID: boundedMessage(client.redactResponseValue(
			firstHeader(header, "X-Request-ID", "X-Correlation-ID"),
		), 256),
	}
}

func (client *Client) redactResponseValue(value string) string {
	return redactErrorValue(value, client.currentErrorSecrets()...)
}

func (client *Client) errorRaw(value []byte) json.RawMessage {
	return boundedRedactedRaw(value, client.currentErrorSecrets()...)
}

func (client *Client) currentErrorSecrets() []string {
	if client.errorSecrets == nil {
		return nil
	}
	return client.errorSecrets()
}

func rawFromAPIError(err error) json.RawMessage {
	var apiError *APIError
	if !errors.As(err, &apiError) {
		return nil
	}
	return append(json.RawMessage(nil), apiError.Raw...)
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
