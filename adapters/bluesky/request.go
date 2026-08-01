package bluesky

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) get(ctx context.Context, method string, query url.Values, output any, options ...socialhub.CallOption) error {
	return c.transport.JSON(ctx, http.MethodGet, "/xrpc/"+method, query, nil, output, options...)
}

func (c *Client) post(ctx context.Context, method string, input, output any, options ...socialhub.CallOption) error {
	return c.transport.JSON(ctx, http.MethodPost, "/xrpc/"+method, nil, input, output, options...)
}

func setPageQuery(query url.Values, cursor string, maximum int) error {
	if maximum < 0 {
		return invalidArgument("pagination", "max results must not be negative")
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if maximum > 0 {
		if maximum > 100 {
			maximum = 100
		}
		query.Set("limit", strconv.Itoa(maximum))
	}
	return nil
}

func pageCursor(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
