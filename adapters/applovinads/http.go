package applovinads

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) query(values url.Values) url.Values {
	result := make(url.Values, len(values)+1)
	for key, entries := range values {
		result[key] = append([]string(nil), entries...)
	}
	result.Set("account_id", client.axonAccountID)
	return result
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	return withOperation(client.api.JSON(ctx, http.MethodGet, path, client.query(query), nil, output, prepared...), operation)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	return withOperation(client.api.JSON(ctx, http.MethodPost, path, client.query(nil), input, output, prepared...), operation)
}

func (client *Client) newRequest(ctx context.Context, operation, method, path string, query url.Values, body io.Reader, options ...socialhub.CallOption) (*http.Request, error) {
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	request, err := client.api.NewRequest(ctx, method, path, client.query(query), body, prepared...)
	return request, withOperation(err, operation)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Axon Campaign Management API does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "Axon Campaign Management API does not document idempotency keys")
	}
	if len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "Axon Campaign Management API does not support field selection")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
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
