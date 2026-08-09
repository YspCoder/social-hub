package outbrain

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodGet, path, query, nil, output, options...), operation)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPost, path, nil, input, output, options...), operation)
}

func (client *Client) putJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPut, path, nil, input, output, options...), operation)
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
