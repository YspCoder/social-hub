package gitlab

import (
	"context"

	"social-hub/pkg/socialhub"
)

// GetAuthenticatedUser returns the user represented by the configured token.
func (client *Client) GetAuthenticatedUser(ctx context.Context, options ...socialhub.CallOption) (*User, ResponseMeta, error) {
	const operation = "get_authenticated_user"
	var user User
	meta, _, err := client.getJSON(ctx, operation, "/user", nil, '{', &user, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validUser(user) {
		return nil, meta, platformContractError(operation, "GitLab returned a user without a valid id, username, or name")
	}
	return &user, meta, nil
}

// GetUser returns one user visible to the signed-in token by global user ID.
func (client *Client) GetUser(ctx context.Context, userID string, options ...socialhub.CallOption) (*User, ResponseMeta, error) {
	const operation = "get_user"
	if !validDecimalID(userID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "user ID is invalid")
	}
	var user User
	meta, _, err := client.getJSON(ctx, operation, "/users/"+userID, nil, '{', &user, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validUser(user) || string(user.ID) != userID {
		return nil, meta, platformContractError(operation, "GitLab returned an absent or mismatched user ID")
	}
	return &user, meta, nil
}
