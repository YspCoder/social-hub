package merchantapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (http.Header, error) {
	if err := client.requireAccess(operation); err != nil {
		return nil, err
	}
	callContext, cancel, err := resolveCallContext(ctx, operation, options)
	if err != nil {
		return nil, err
	}
	defer cancel()
	request, err := client.api.NewRequest(callContext, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	request.Header.Set("Accept-Encoding", "identity")
	metadata, err := client.api.DoWithMetadata(request, output)
	if err != nil {
		return metadata.Header, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return metadata.Header, platformContractError(operation, "Merchant API returned an unexpected successful HTTP status")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return metadata.Header, platformContractError(operation, "Merchant API success response was not JSON")
	}
	return metadata.Header, nil
}

func (client *Client) sendJSON(ctx context.Context, operation, method, path string, query url.Values, input, output any, mutation bool, options ...socialhub.CallOption) (http.Header, error) {
	if err := client.requireAccess(operation); err != nil {
		return nil, err
	}
	callContext, cancel, err := resolveCallContext(ctx, operation, options)
	if err != nil {
		return nil, err
	}
	defer cancel()
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(callContext, method, path, query, body)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept-Encoding", "identity")
	metadata, err := client.api.DoWithMetadata(request, output)
	if err != nil {
		err = withOperation(err, operation)
		if mutation && (metadata.StatusCode >= 200 && metadata.StatusCode < 300 || ambiguousMutationError(err)) {
			return metadata.Header, outcomeUnknownError(operation, err, responseRequestID(metadata.Header, client.requestIDs), client.requestIDs)
		}
		return metadata.Header, err
	}
	expectedStatus := metadata.StatusCode == http.StatusOK || output == nil && metadata.StatusCode == http.StatusNoContent
	if !expectedStatus {
		err := platformContractError(operation, "Merchant API returned an unexpected successful HTTP status")
		if mutation {
			return metadata.Header, outcomeUnknownError(operation, err, responseRequestID(metadata.Header, client.requestIDs), client.requestIDs)
		}
		return metadata.Header, err
	}
	if output != nil && !validJSONContentType(metadata.Header.Get("Content-Type")) {
		err := platformContractError(operation, "Merchant API success response was not JSON")
		if mutation {
			return metadata.Header, outcomeUnknownError(operation, err, responseRequestID(metadata.Header, client.requestIDs), client.requestIDs)
		}
		return metadata.Header, err
	}
	return metadata.Header, nil
}

func resolveCallContext(ctx context.Context, operation string, options []socialhub.CallOption) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, invalidArgument(operation, "context is required")
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, nil, invalidArgument(operation, "Merchant API methods do not accept a caller request ID")
	}
	if resolved.IdempotencyKey != "" {
		return nil, nil, invalidArgument(operation, "Merchant API methods do not document an idempotency key")
	}
	if len(resolved.Fields) > 0 {
		return nil, nil, invalidArgument(operation, "generic response fields are unsupported because identity fields are required for account-bound validation")
	}
	if resolved.Timeout < 0 {
		return nil, nil, invalidArgument(operation, "timeout must not be negative")
	}
	if resolved.Timeout > 0 {
		callContext, cancel := context.WithTimeout(ctx, resolved.Timeout)
		return callContext, cancel, nil
	}
	return ctx, func() {}, nil
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}
