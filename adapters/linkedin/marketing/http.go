package marketing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(ctx context.Context, operation, path, rawQuery, restliMethod string, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireRead(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.readJSON(ctx, path, rawQuery, restliMethod, output, options...)
}

func (client *Client) reportJSON(ctx context.Context, operation, path, rawQuery string, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireReporting(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.readJSON(ctx, path, rawQuery, "", output, options...)
}

func (client *Client) readJSON(ctx context.Context, path, rawQuery, restliMethod string, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, nil, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	request.URL.RawQuery = rawQuery
	if restliMethod != "" {
		request.Header.Set("X-RestLi-Method", restliMethod)
	}
	return client.api.DoWithMetadata(request, output)
}

func (client *Client) writeJSON(ctx context.Context, operation, path, restliMethod string, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireWrite(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return transport.ResponseMetadata{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if restliMethod != "" {
		request.Header.Set("X-RestLi-Method", restliMethod)
	}
	return client.api.DoWithMetadata(request, output)
}
