package threads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) get(ctx context.Context, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return c.transport.JSON(ctx, http.MethodGet, path, query, nil, output, options...)
}

func (c *Client) form(ctx context.Context, method, path string, form url.Values, output any, options ...socialhub.CallOption) error {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	request, err := c.transport.NewRequest(ctx, method, path, nil, body, options...)
	if err != nil {
		return err
	}
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		request.ContentLength = 0
	}
	return c.transport.Do(request, output)
}

func setPaging(query url.Values, cursor string, maximum int) error {
	if maximum < 0 {
		return invalidArgument("pagination", "max results must not be negative")
	}
	if cursor != "" {
		query.Set("after", cursor)
	}
	if maximum > 0 {
		if maximum > 100 {
			maximum = 100
		}
		query.Set("limit", strconv.Itoa(maximum))
	}
	return nil
}

func validToken(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, character := range value {
		if !(character == '_' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
			return false
		}
	}
	return true
}
