package applemusic

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	body := bytes.NewReader(nil)
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return transport.ResponseMetadata{}, platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := c.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return c.api.DoWithMetadata(request, output)
}

func getResource[T any](ctx context.Context, client *Client, operation, path string, query url.Values, options ...socialhub.CallOption) (*T, error) {
	var response apiCollection[T]
	if _, err := client.requestJSON(ctx, http.MethodGet, path, query, nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Data[0], nil
}
