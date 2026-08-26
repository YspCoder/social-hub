package rakutenadvertising

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxRequestBytes  = 1 << 20
	maxResponseBytes = int64(8 << 20)
)

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
	responseMeta := ResponseMeta{RequestID: boundedMessage(client.redactResponseValue(firstNonEmpty(
		firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID,
	)), 256)}
	rawCopy := append(json.RawMessage(nil), raw...)
	if err != nil {
		if providerRaw := rawFromAPIError(err); len(providerRaw) > 0 {
			rawCopy = providerRaw
		}
		return responseMeta, rawCopy, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Rakuten Advertising returned an unexpected successful HTTP status", metadata.StatusCode,
		)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Rakuten Advertising returned an empty or invalid JSON success response", metadata.StatusCode,
		)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Rakuten Advertising returned a non-JSON success response", metadata.StatusCode,
		)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return responseMeta, client.errorRaw(rawCopy), withHTTPStatus(
			platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err), metadata.StatusCode,
		)
	}
	return responseMeta, rawCopy, nil
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (ResponseMeta, json.RawMessage, error) {
	return client.doJSON(ctx, operation, http.MethodGet, path, query, nil, output, options...)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) (ResponseMeta, json.RawMessage, error) {
	return client.doJSON(ctx, operation, http.MethodPost, path, nil, input, output, options...)
}

func (client *Client) getXML(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output *ProductSearchResponse,
	options ...socialhub.CallOption,
) (ResponseMeta, []byte, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/xml, text/xml")
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	metadata := ResponseMeta{RequestID: boundedMessage(client.redactResponseValue(firstNonEmpty(
		firstHeader(response.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID,
	)), 256)}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return metadata, client.errorRaw(body), withHTTPStatus(
			platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err), response.StatusCode,
		)
	}
	if int64(len(body)) > maxResponseBytes {
		return metadata, client.errorRaw(body), platformContractError(operation, "Rakuten Advertising response exceeded 8 MiB", response.StatusCode)
	}
	raw := append([]byte(nil), body...)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return metadata, client.errorRaw(raw), withOperation(client.decodeResponseError(response.StatusCode, response.Header, body), operation)
	}
	if response.StatusCode != http.StatusOK {
		return metadata, client.errorRaw(raw), platformContractError(
			operation, "Rakuten Advertising returned an unexpected Product Search success status", response.StatusCode,
		)
	}
	if !validXMLContentType(response.Header.Get("Content-Type")) {
		return metadata, client.errorRaw(raw), platformContractError(
			operation, "Rakuten Advertising returned a non-XML Product Search success response", response.StatusCode,
		)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return metadata, client.errorRaw(raw), platformContractError(operation, "Rakuten Advertising returned an empty Product Search response", response.StatusCode)
	}
	if provider := decodeProviderError(body); provider.Code != "" {
		return metadata, client.errorRaw(raw), withOperation(client.decodeResponseError(response.StatusCode, response.Header, body), operation)
	}
	if err := xml.Unmarshal(body, output); err != nil {
		if provider := decodeProviderError(body); provider.Code != "" || provider.Message != "" {
			return metadata, client.errorRaw(raw), withOperation(client.decodeResponseError(response.StatusCode, response.Header, body), operation)
		}
		return metadata, client.errorRaw(raw), withHTTPStatus(
			platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err), response.StatusCode,
		)
	}
	if output.XMLName.Local != "result" {
		return metadata, client.errorRaw(raw), platformContractError(
			operation, "Rakuten Advertising returned an unexpected Product Search XML root", response.StatusCode,
		)
	}
	return metadata, raw, nil
}

func (client *Client) decodeResponseError(status int, header http.Header, body []byte) error {
	if client.decodeError != nil {
		return client.decodeError(status, header, body)
	}
	return decodeHTTPError(status, header, body, client.clock.Now())
}

func (client *Client) redactResponseValue(value string) string {
	return redactErrorValue(value, client.currentErrorSecrets()...)
}

func (client *Client) errorRaw(value []byte) []byte {
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

func validXMLContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/xml" || mediaType == "text/xml" || strings.HasSuffix(mediaType, "+xml")
}
