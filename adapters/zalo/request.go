package zalo

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type apiEnvelope[T any] struct {
	Data    T      `json:"data"`
	Error   int    `json:"error"`
	Message string `json:"message"`
}

func request[T any](ctx context.Context, client *Client, method, path string, query url.Values, input any, operation string, options ...socialhub.CallOption) (T, error) {
	var envelope apiEnvelope[T]
	if err := client.api.JSON(ctx, method, path, query, input, &envelope, options...); err != nil {
		return envelope.Data, err
	}
	if envelope.Error != 0 {
		return envelope.Data, mapAPIError(operation, envelope.Error, envelope.Message)
	}
	return envelope.Data, nil
}

func get[T any](ctx context.Context, client *Client, path string, query url.Values, operation string, options ...socialhub.CallOption) (T, error) {
	return request[T](ctx, client, http.MethodGet, path, query, nil, operation, options...)
}
