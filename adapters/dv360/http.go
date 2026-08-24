package dv360

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func listQuery(input ListRequest) url.Values {
	query := make(url.Values)
	if input.PageSize > 0 {
		query.Set("pageSize", strconv.Itoa(input.PageSize))
	}
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}
	if input.Filter != "" {
		query.Set("filter", input.Filter)
	}
	if input.OrderBy != "" {
		query.Set("orderBy", input.OrderBy)
	}
	return query
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	if err := client.requireAccess(operation); err != nil {
		return err
	}
	return withOperation(client.api.JSON(ctx, http.MethodGet, path, query, nil, output, options...), operation)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	if err := client.requireAccess(operation); err != nil {
		return err
	}
	return withOperation(client.api.JSON(ctx, http.MethodPost, path, nil, input, output, options...), operation)
}

func (client *Client) patchJSON(ctx context.Context, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	if err := client.requireAccess(operation); err != nil {
		return err
	}
	return withOperation(client.api.JSON(ctx, http.MethodPatch, path, query, input, output, options...), operation)
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
