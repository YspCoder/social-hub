package wordpresscom

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) sitePath(parts ...string) string {
	path := "/sites/" + url.PathEscape(client.site)
	for _, part := range parts {
		path += "/" + url.PathEscape(part)
	}
	return path
}

func (client *Client) form(ctx context.Context, api *transport.Client, path string, values url.Values, output any, options ...socialhub.CallOption) error {
	encoded := ""
	if values != nil {
		encoded = values.Encode()
	}
	request, err := api.NewRequest(ctx, http.MethodPost, path, nil, strings.NewReader(encoded), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return api.Do(request, output)
}

func pageQuery(cursor string, maximum int, start, end *time.Time) (url.Values, error) {
	if maximum < 0 {
		return nil, invalidArgument("pagination", "max_results must not be negative")
	}
	if cursor != "" && !validCursor(cursor) {
		return nil, invalidArgument("pagination", "page_handle cursor is invalid")
	}
	if maximum == 0 {
		maximum = 20
	}
	if maximum > 100 {
		maximum = 100
	}
	query := url.Values{"number": {strconv.Itoa(maximum)}}
	if cursor != "" {
		query.Set("page_handle", cursor)
	}
	if start != nil {
		query.Set("after", start.UTC().Format(time.RFC3339))
	}
	if end != nil {
		query.Set("before", end.UTC().Format(time.RFC3339))
	}
	return query, nil
}

func validCursor(value string) bool {
	if value == "" || len(value) > 4096 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func mustID(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}
