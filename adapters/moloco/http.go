package moloco

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
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
	responseMetadata := client.responseMeta(metadata)
	if err != nil {
		return responseMetadata, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return responseMetadata, platformContractError(
			operation, "Moloco returned an unexpected successful HTTP status", metadata.StatusCode,
		)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return responseMetadata, platformContractError(
			operation, "Moloco returned an empty or invalid JSON response", metadata.StatusCode,
		)
	}
	if !jsonContentType(metadata.Header.Get("Content-Type")) {
		return responseMetadata, platformContractError(
			operation, "Moloco returned a non-JSON response", metadata.StatusCode,
		)
	}
	if output != nil && json.Unmarshal(raw, output) != nil {
		return responseMetadata, platformContractError(
			operation, "Moloco returned JSON with invalid report fields", metadata.StatusCode,
		)
	}
	return responseMetadata, nil
}

func (client *Client) responseMeta(metadata transport.ResponseMetadata) ResponseMeta {
	redact := func(value string, maximum int) string {
		return boundedMessage(redactErrorValue(value, client.tokens.redactionSecrets()...), maximum)
	}
	return ResponseMeta{
		RequestID:          redact(firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"), 256),
		RateLimitQuota:     redact(metadata.Header.Get("X-Rate-Limit-Quota"), 64),
		RateLimitRemaining: redact(metadata.Header.Get("X-Rate-Limit-Remaining"), 64),
		RateLimitReset:     redact(metadata.Header.Get("X-Rate-Limit-Reset"), 64),
	}
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var apiError *APIError
	if errors.As(err, &apiError) && apiError.Hub != nil {
		apiError.Hub.Op = operation
		apiError.Hub.Platform = platformName
		apiError.Hub.Product = productName
		return apiError
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		var nested *socialhub.Error
		if errors.As(hub.Cause, &nested) && nested != hub {
			nested.Op = operation
			nested.Platform = platformName
			nested.Product = productName
			return nested
		}
		hub.Op = operation
		hub.Platform = platformName
		hub.Product = productName
		return hub
	}
	return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
}
