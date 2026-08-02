package deviantart

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) request(ctx context.Context, method, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return client.execute(ctx, method, path, query, nil, "", output, options...)
}

func (client *Client) form(ctx context.Context, path string, values url.Values, output any, options ...socialhub.CallOption) error {
	return client.execute(ctx, http.MethodPost, path, nil, strings.NewReader(values.Encode()), "application/x-www-form-urlencoded", output, options...)
}

func (client *Client) execute(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string, output any, options ...socialhub.CallOption) error {
	api, err := client.requireAPI(method + " " + path)
	if err != nil {
		return err
	}
	request, err := api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("dA-minor-version", minorVersion)
	request.Header.Set("User-Agent", client.userAgent)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return api.Do(request, output)
}

func apiPath(parts ...string) string {
	return "/" + strings.Join(parts, "/")
}
