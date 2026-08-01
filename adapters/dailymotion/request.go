package dailymotion

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return c.api.JSON(ctx, method, path, query, input, output, options...)
}

func escapedID(value string) string { return url.PathEscape(value) }
