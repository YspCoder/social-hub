package analyticsdata

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) jsonRequest(ctx context.Context, method, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	if err := client.requireReadScope(operation); err != nil {
		return err
	}
	return withOperation(client.api.JSON(ctx, method, path, query, input, output, options...), operation)
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return client.jsonRequest(ctx, http.MethodGet, operation, path, query, nil, output, options...)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return client.jsonRequest(ctx, http.MethodPost, operation, path, nil, input, output, options...)
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
