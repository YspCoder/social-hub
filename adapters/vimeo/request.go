package vimeo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"

	"social-hub/pkg/socialhub"
)

const vimeoAccept = "application/vnd.vimeo.*+json;version=3.4"

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
	request.Header.Set("Accept", vimeoAccept)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return c.api.Do(request, output)
}

func escapedID(value string) string { return url.PathEscape(value) }
