package amazonads

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(ctx context.Context, operation, path, accept string, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireAccess(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, nil, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	return client.api.DoWithMetadata(request, output)
}

func (client *Client) vendorJSON(ctx context.Context, operation, method, path, mediaType string, input, output any, prefer bool, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireAccess(operation); err != nil {
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
	request.Header.Set("Accept", mediaType)
	request.Header.Set("Content-Type", mediaType)
	if prefer {
		request.Header.Set("Prefer", "return=representation")
	}
	return client.api.DoWithMetadata(request, output)
}
