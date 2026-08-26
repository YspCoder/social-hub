package tidal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxResponseBytes int64 = 8 << 20

func (client *Client) get(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	options ...socialhub.CallOption,
) (json.RawMessage, ResponseMeta, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	requestURL := defaultBaseURL + "/" + strings.TrimLeft(path, "/")
	if encoded := query.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/vnd.api+json")
	request.Header.Set("Authorization", "Bearer "+client.accessToken)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, ResponseMeta{}, credentialPlatformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err, client.accessToken)
	}
	defer response.Body.Close()
	meta := responseMeta(response.StatusCode, response.Header, client.accessToken)
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, meta, credentialPlatformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err, client.accessToken)
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, meta, platformContractError(operation, "response exceeded the 8 MiB size limit")
	}
	if response.StatusCode != http.StatusOK {
		return nil, meta, decodeHTTPError(operation, response.StatusCode, response.Header, body, client.accessToken, client.clock)
	}
	if !validSuccessContentType(response.Header.Get("Content-Type")) {
		return nil, meta, platformContractError(operation, "TIDAL returned a non-JSON:API success response")
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || !utf8.Valid(trimmed) {
		return nil, meta, platformContractError(operation, "TIDAL returned an empty or invalid JSON success response")
	}
	var document any
	if err := json.Unmarshal(trimmed, &document); err != nil {
		return nil, meta, platformContractError(operation, "TIDAL returned an empty or invalid JSON success response")
	}
	if containsJSONSecret(document, client.accessToken) {
		return nil, meta, platformContractError(operation, "TIDAL returned bearer-token material in a success response")
	}
	return cloneRaw(trimmed), meta, nil
}

func validSuccessContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/vnd.api+json")
}

func responseMeta(status int, header http.Header, token string) ResponseMeta {
	clean := func(value string, maximum int) string {
		return safeHeader(value, token, maximum)
	}
	return ResponseMeta{
		StatusCode:         status,
		RetryAfter:         clean(header.Get("Retry-After"), 256),
		RateLimitLimit:     clean(header.Get("X-RateLimit-Limit"), 256),
		RateLimitRemaining: clean(header.Get("X-RateLimit-Remaining"), 256),
		RateLimitReset:     clean(header.Get("X-RateLimit-Reset"), 256),
		CloudFrontID:       clean(header.Get("X-Amz-Cf-Id"), 512),
		RequestID:          clean(header.Get("X-Request-ID"), 256),
		ETag:               clean(header.Get("ETag"), 1024),
		CacheControl:       clean(header.Get("Cache-Control"), 1024),
		Deprecation:        clean(header.Get("Deprecation"), 1024),
		Sunset:             clean(header.Get("Sunset"), 1024),
		Warning:            clean(header.Get("Warning"), 1024),
		LastModified:       clean(header.Get("Last-Modified"), 1024),
	}
}

func containsJSONSecret(value any, secret string) bool {
	if secret == "" {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.Contains(typed, secret)
	case []any:
		for _, child := range typed {
			if containsJSONSecret(child, secret) {
				return true
			}
		}
	case map[string]any:
		for key, child := range typed {
			if strings.Contains(key, secret) || containsJSONSecret(child, secret) {
				return true
			}
		}
	}
	return false
}

type pageEnvelope[T any] struct {
	Data     json.RawMessage    `json:"data"`
	Included []IncludedResource `json:"included,omitempty"`
	Links    Links              `json:"links"`
	Meta     json.RawMessage    `json:"meta,omitempty"`
}

type documentEnvelope[T any] struct {
	Data     json.RawMessage    `json:"data"`
	Included []IncludedResource `json:"included,omitempty"`
	Links    Links              `json:"links"`
	Meta     json.RawMessage    `json:"meta,omitempty"`
}

func decodePage[T any](
	operation string,
	endpointPath string,
	body json.RawMessage,
	response ResponseMeta,
	expectedType string,
	identity func(*T) (string, string),
) (*Page[T], error) {
	var envelope pageEnvelope[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	trimmedData := bytes.TrimSpace(envelope.Data)
	if len(trimmedData) == 0 || trimmedData[0] != '[' {
		return nil, platformContractError(operation, "JSON:API collection data must be an array")
	}
	var items []T
	if err := json.Unmarshal(trimmedData, &items); err != nil {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := validateEnvelope(operation, envelope.Links, envelope.Included); err != nil {
		return nil, err
	}
	for index := range items {
		resourceType, id := identity(&items[index])
		if resourceType != expectedType || !validOpaque(id, maxIDLength) {
			return nil, platformContractError(operation, "JSON:API data contains an invalid resource type or id")
		}
	}
	cursor, err := nextCursor(envelope.Links, endpointPath)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	return &Page[T]{
		Items: items, Included: envelope.Included, Links: envelope.Links,
		NextCursor: cursor, Meta: cloneRaw(envelope.Meta), Response: response, Raw: cloneRaw(body),
	}, nil
}

func decodeDocument[T any](
	operation string,
	body json.RawMessage,
	response ResponseMeta,
	expectedType string,
	identity func(*T) (string, string),
) (*Document[T], error) {
	var envelope documentEnvelope[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	trimmedData := bytes.TrimSpace(envelope.Data)
	if len(trimmedData) == 0 || trimmedData[0] != '{' {
		return nil, platformContractError(operation, "JSON:API document data must be one resource object")
	}
	var item T
	if err := json.Unmarshal(trimmedData, &item); err != nil {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := validateEnvelope(operation, envelope.Links, envelope.Included); err != nil {
		return nil, err
	}
	resourceType, id := identity(&item)
	if resourceType != expectedType || !validOpaque(id, maxIDLength) {
		return nil, platformContractError(operation, "JSON:API data contains an invalid resource type or id")
	}
	return &Document[T]{
		Item: item, Included: envelope.Included, Links: envelope.Links,
		Meta: cloneRaw(envelope.Meta), Response: response, Raw: cloneRaw(body),
	}, nil
}

func validateEnvelope(operation string, links Links, included []IncludedResource) error {
	if !validDocumentLink(links.Self) {
		return platformContractError(operation, "JSON:API links.self is missing or untrusted")
	}
	for _, resource := range included {
		if !validOpaque(resource.Type, 256) || !validOpaque(resource.ID, maxIDLength) {
			return platformContractError(operation, "JSON:API included contains an invalid resource type or id")
		}
	}
	return nil
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}
