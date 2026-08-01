package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	var body *bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return transport.ResponseMetadata{}, platformErrorWithCause(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
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

func escapedID(value string) string { return url.PathEscape(value) }
