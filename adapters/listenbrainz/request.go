package listenbrainz

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, operation, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	if err := validateCallOptions(operation, options...); err != nil {
		return err
	}
	var err error
	if input != nil {
		err = c.api.JSON(ctx, method, path, query, input, output, options...)
	} else {
		request, requestErr := c.api.NewRequest(ctx, method, path, query, nil, options...)
		if requestErr != nil {
			return requestErr
		}
		_, err = c.api.DoWithMetadata(request, output)
	}
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return err
}

func getOnly(ctx context.Context, c *Client, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return c.requestJSON(ctx, operation, http.MethodGet, path, query, nil, output, options...)
}
