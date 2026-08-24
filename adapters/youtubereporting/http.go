package youtubereporting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) jsonRequest(
	ctx context.Context,
	method, operation, path string,
	query url.Values,
	input, output any,
	options ...socialhub.CallOption,
) (transport.ResponseMetadata, error) {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return transport.ResponseMetadata{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return transport.ResponseMetadata{}, withOperation(err, operation)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	metadata, err := client.api.DoWithMetadata(request, output)
	return metadata, withOperation(err, operation)
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	_, err := client.jsonRequest(ctx, http.MethodGet, operation, path, query, nil, output, options...)
	return err
}

func (client *Client) postJSON(ctx context.Context, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	_, err := client.jsonRequest(ctx, http.MethodPost, operation, path, query, input, output, options...)
	return err
}

func (client *Client) deleteJSON(ctx context.Context, operation, path string, query url.Values, options ...socialhub.CallOption) error {
	_, err := client.jsonRequest(ctx, http.MethodDelete, operation, path, query, nil, nil, options...)
	return err
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
