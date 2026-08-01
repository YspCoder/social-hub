package spotify

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) CurrentUser(ctx context.Context, options ...socialhub.CallOption) (*socialhub.User, error) {
	var response spotifyPrivateUser
	if _, err := c.requestJSON(ctx, http.MethodGet, "/me", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return c.mapUser(response)
}

var _ ProfileWorkflow = (*Client)(nil)
