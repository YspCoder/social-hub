package dingtalk

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) call(ctx context.Context, operation, method, path string, input, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return unsupported(operation, "field selection is not supported by this DingTalk operation")
	}
	if resolved.IdempotencyKey != "" {
		return unsupported(operation, "this DingTalk operation does not document an idempotency key")
	}
	clean := make([]socialhub.CallOption, 0, 2)
	if resolved.RequestID != "" {
		clean = append(clean, socialhub.WithRequestID(resolved.RequestID))
	}
	if resolved.Timeout > 0 {
		clean = append(clean, socialhub.WithCallTimeout(resolved.Timeout))
	}
	if err := c.api.JSON(ctx, method, path, url.Values{}, input, output, clean...); err != nil {
		if platformErr, ok := err.(*socialhub.Error); ok {
			platformErr.Op = operation
		}
		return err
	}
	return nil
}

func (c *Client) get(ctx context.Context, operation, path string, output any, options ...socialhub.CallOption) error {
	return c.call(ctx, operation, http.MethodGet, path, nil, output, options...)
}
