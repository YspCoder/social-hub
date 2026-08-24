package bitbucket

import (
	"context"

	"social-hub/pkg/socialhub"
)

// GetCurrentUser returns the account represented by the configured credential.
func (client *Client) GetCurrentUser(ctx context.Context, options ...socialhub.CallOption) (*Account, ResponseMeta, error) {
	const operation = "get_current_user"
	var account Account
	meta, _, err := client.getJSON(ctx, operation, "/user", nil, &account, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validAccount(account) {
		return nil, meta, platformContractError(operation, "Bitbucket returned an account without a valid uuid")
	}
	return &account, meta, nil
}
