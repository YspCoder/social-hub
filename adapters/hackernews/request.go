package hackernews

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) getJSON(ctx context.Context, operation, path string, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return unsupported(operation, "field selection is not supported by Hacker News API v0")
	}
	if resolved.IdempotencyKey != "" {
		return unsupported(operation, "read-only Hacker News requests do not use idempotency keys")
	}
	clean := make([]socialhub.CallOption, 0, 2)
	if resolved.RequestID != "" {
		clean = append(clean, socialhub.WithRequestID(resolved.RequestID))
	}
	if resolved.Timeout > 0 {
		clean = append(clean, socialhub.WithCallTimeout(resolved.Timeout))
	}
	request, err := c.api.NewRequest(ctx, http.MethodGet, path, url.Values{}, nil, clean...)
	if err == nil {
		_, err = c.api.DoWithMetadata(request, output)
	}
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return err
}
