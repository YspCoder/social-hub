package naversearchads

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

func (client *Client) doJSON(
	ctx context.Context,
	operation, method, path string,
	query url.Values,
	input, output any,
	mutation bool,
	options ...socialhub.CallOption,
) (transport.ResponseMetadata, error) {
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return transport.ResponseMetadata{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return transport.ResponseMetadata{}, invalidArgument(operation, "request JSON exceeds 1 MiB")
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, prepared...)
	if err != nil {
		return transport.ResponseMetadata{}, withMutationOutcome(operation, mutation, err)
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}
	metadata, err := client.api.DoWithMetadata(request, output)
	if err != nil {
		return metadata, withMutationOutcome(operation, mutation, err)
	}
	return metadata, nil
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	_, err := client.doJSON(ctx, operation, http.MethodGet, path, query, nil, output, false, options...)
	return err
}

func (client *Client) writeJSON(ctx context.Context, operation, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	_, err := client.doJSON(ctx, operation, method, path, query, input, output, true, options...)
	return err
}

func (client *Client) delete(ctx context.Context, operation, path string, options ...socialhub.CallOption) error {
	_, err := client.doJSON(ctx, operation, http.MethodDelete, path, nil, nil, nil, true, options...)
	return err
}

func withMutationOutcome(operation string, mutation bool, err error) error {
	err = withOperation(err, operation)
	if mutation && ambiguousMutationError(err) {
		return outcomeUnknownError(operation, err)
	}
	return err
}
