package whatsapp

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) request(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	return c.transport.JSON(ctx, method, path, query, input, output, options...)
}

func (c *Client) phonePath(edge string) string {
	return "/" + url.PathEscape(c.phoneNumberID) + "/" + edge
}

type successPayload struct {
	Success bool `json:"success"`
}

func requireSuccess(output successPayload, operation string) error {
	if !output.Success {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}
