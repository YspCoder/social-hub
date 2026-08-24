package github

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
		return nil, meta, platformContractError(operation, "GitHub returned a user without a valid id or login")
	}
	return &user, meta, nil
}

// GetUser returns one public user profile by login.
func (client *Client) GetUser(ctx context.Context, username string, options ...socialhub.CallOption) (*User, ResponseMeta, error) {
	const operation = "get_user"
	if !validPathSegment(username) {
		return nil, ResponseMeta{}, invalidArgument(operation, "username is invalid")
	}
	var user User
	meta, _, err := client.getJSON(ctx, operation, "/users/"+username, nil, '{', &user, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validUser(user) {
		return nil, meta, platformContractError(operation, "GitHub returned a user without a valid id or login")
	}
	return &user, meta, nil
}
