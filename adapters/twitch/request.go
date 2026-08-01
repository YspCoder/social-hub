package twitch

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (c *Client) get(ctx context.Context, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return c.transport.JSON(ctx, http.MethodGet, path, query, nil, output, options...)
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return c.transport.JSON(ctx, method, path, query, input, output, options...)
}

func appRequest(ctx context.Context, client *transport.Client, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return client.JSON(ctx, method, path, query, input, output, options...)
}

func setPaging(query url.Values, cursor string, maximum int) error {
	if maximum < 0 {
		return invalidArgument("pagination", "max results must not be negative")
	}
	if strings.TrimSpace(cursor) != "" {
		query.Set("after", cursor)
	}
	if maximum > 0 {
		if maximum > 100 {
			maximum = 100
		}
		query.Set("first", strconv.Itoa(maximum))
	}
	return nil
}

func appendValues(query url.Values, key string, values []string, maximum int) error {
	if len(values) > maximum {
		return invalidArgument("request", key+" exceeds the platform maximum")
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return invalidArgument("request", key+" values must not be empty")
		}
		query.Add(key, value)
	}
	return nil
}
