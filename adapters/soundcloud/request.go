package soundcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	var body *bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := c.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	return c.api.Do(request, output)
}

func escapedURN(value string) string { return url.PathEscape(value) }
