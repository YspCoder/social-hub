package letterboxd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	body := bytes.NewReader(nil)
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := c.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	err = c.api.Do(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = method + " " + path
	}
	return err
}

func pageQuery(cursor string, perPage int) url.Values {
	query := url.Values{}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if perPage > 0 {
		query.Set("perPage", strconv.Itoa(perPage))
	}
	return query
}

type pageEnvelope[T any] struct {
	Next  string `json:"next"`
	Items []T    `json:"items"`
}

func toPage[T any](items []T, next string) socialhub.Page[T] {
	result := socialhub.Page[T]{Items: items, HasMore: next != ""}
	if next != "" {
		result.NextCursor = &next
	}
	return result
}

func escaped(value string) string { return url.PathEscape(value) }

var _ transport.ErrorDecoder = decodeHTTPError
