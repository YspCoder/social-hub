package ads

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireRead(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.api.DoWithMetadata(request, output)
}

func (client *Client) writeJSON(ctx context.Context, operation, method, path string, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireWrite(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return transport.ResponseMetadata{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, method, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	return client.api.DoWithMetadata(request, output)
}
