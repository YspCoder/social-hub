package cm360

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) jsonRequest(
	ctx context.Context,
	method, operation, path string,
	query url.Values,
	input, output any,
	requiredScope string,
	options ...socialhub.CallOption,
) error {
	if err := client.requireScope(operation, requiredScope); err != nil {
		return err
	}
	return withOperation(client.api.JSON(ctx, method, path, query, input, output, options...), operation)
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, scope string, options ...socialhub.CallOption) error {
	return client.jsonRequest(ctx, http.MethodGet, operation, path, query, nil, output, scope, options...)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, query url.Values, input, output any, scope string, options ...socialhub.CallOption) error {
	return client.jsonRequest(ctx, http.MethodPost, operation, path, query, input, output, scope, options...)
}

func (client *Client) patchJSON(ctx context.Context, operation, path string, query url.Values, input, output any, scope string, options ...socialhub.CallOption) error {
	return client.jsonRequest(ctx, http.MethodPatch, operation, path, query, input, output, scope, options...)
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

func (client *Client) profilePath() string {
	return "/userprofiles/" + client.profileID
}
