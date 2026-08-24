package appleads

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type responseEnvelope[T any] struct {
	Data       T          `json:"data"`
	Pagination PageDetail `json:"pagination"`
	Error      errorBody  `json:"error"`
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodGet, path, query, nil, output, options...), operation)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPost, path, nil, input, output, options...), operation)
}

func (client *Client) putJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPut, path, nil, input, output, options...), operation)
}

func (client *Client) deleteJSON(ctx context.Context, operation, path string, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodDelete, path, nil, nil, output, options...), operation)
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

func checkEnvelopeError(operation string, value errorBody) error {
	if len(value.Errors) == 0 {
		return nil
	}
	return businessError(operation, value.Errors)
}

func pageResult[T any](items []T, detail PageDetail) Page[T] {
	return Page[T]{
		Items: items, Pagination: detail,
		HasMore: detail.StartIndex+detail.ItemsPerPage < int(detail.TotalResults),
	}
}

func listQuery(value Pagination) url.Values {
	return url.Values{"offset": {strconv.Itoa(value.Offset)}, "limit": {strconv.Itoa(value.Limit)}}
}
