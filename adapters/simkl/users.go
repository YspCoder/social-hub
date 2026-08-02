package simkl

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

// GetSettings reads current-user settings. Simkl retains POST for this read-only endpoint.
func (c *Client) GetSettings(ctx context.Context, options ...socialhub.CallOption) (*UserSettings, error) {
	if err := c.requireOAuth("user_settings"); err != nil {
		return nil, err
	}
	var response UserSettings
	if _, err := requestJSON(ctx, c.userAPI, "user_settings", http.MethodPost, "/users/settings", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}
