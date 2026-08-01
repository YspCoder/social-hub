package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) request(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return wrapError("request", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := c.transport.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", c.userAgent)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return c.transport.Do(request, output)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return c.request(ctx, http.MethodGet, path, query, nil, output, options...)
}
