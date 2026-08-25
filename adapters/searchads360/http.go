package searchads360

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(ctx context.Context, operation, path string, output any, options ...socialhub.CallOption) (http.Header, error) {
	if err := client.requireAccess(operation); err != nil {
		return nil, err
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, nil, nil, prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	metadata, err := client.api.DoWithMetadata(request, output)
	return metadata.Header, withOperation(err, operation)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) (http.Header, error) {
	if err := client.requireAccess(operation); err != nil {
		return nil, err
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), prepared...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.api.DoWithMetadata(request, output)
	return metadata.Header, withOperation(err, operation)
}
