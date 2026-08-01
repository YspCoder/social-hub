package kakao

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) form(ctx context.Context, path string, values url.Values, output any, options ...socialhub.CallOption) error {
	request, err := c.api.NewRequest(ctx, http.MethodPost, path, nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	return c.api.Do(request, output)
}
