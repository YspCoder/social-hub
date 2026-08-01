package imgur

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

// Credits returns the current application and user credit counters.
func (client *Client) Credits(ctx context.Context, options ...socialhub.CallOption) (*Credits, error) {
	var credits Credits
	if err := client.request(ctx, client.active(), http.MethodGet, path("credits"), nil, &credits, options...); err != nil {
		return nil, err
	}
	if credits.UserLimit < 0 || credits.UserRemaining < 0 || credits.UserReset < 0 || credits.ClientLimit < 0 || credits.ClientRemaining < 0 {
		return nil, platformError("credits", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &credits, nil
}

var _ CreditWorkflow = (*Client)(nil)
