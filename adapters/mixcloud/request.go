package mixcloud

import (
	"context"
	"io"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) request(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string, output any, options ...socialhub.CallOption) error {
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", client.userAgent)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return client.api.Do(request, output)
}
