package unitypublisher

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

func (client *Client) postJSON(ctx context.Context, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPost, path, query, input, output, options...), operation)
}

func (client *Client) patchJSON(ctx context.Context, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPatch, path, query, input, output, options...), operation)
}

func (client *Client) putJSON(ctx context.Context, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPut, path, query, input, output, options...), operation)
}

func (client *Client) deleteJSON(ctx context.Context, operation, path string, query url.Values, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodDelete, path, query, nil, nil, options...), operation)
}

func mutationQuery(options MutationOptions) url.Values {
	if !options.DryRun {
		return nil
	}
	return url.Values{"dryrun": {"true"}}
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
