package myanimelist

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

type paging struct {
	Previous string `json:"previous"`
	Next     string `json:"next"`
}

type nodeEnvelope[T any] struct {
	Node T `json:"node"`
}

type nodePageEnvelope[T any] struct {
	Data   []nodeEnvelope[T] `json:"data"`
	Paging paging            `json:"paging"`
}

type pageEnvelope[T any] struct {
	Data   []T    `json:"data"`
	Paging paging `json:"paging"`
}

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	var err error
	query, err = queryWithFields(query, options...)
	if err != nil {
		return err
	}
	request, err := c.api.NewRequest(ctx, method, path, query, nil, options...)
	if err != nil {
		return err
	}
	err = c.api.Do(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = method + " " + path
		platformErr.Cause = sanitizeTransportError(platformErr.Cause)
	}
	return err
}

func (c *Client) requestForm(ctx context.Context, method, path string, values url.Values, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(method+" "+path, "fields are not supported for list mutations")
	}
	request, err := c.api.NewRequest(ctx, method, path, nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	err = c.api.Do(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = method + " " + path
		platformErr.Cause = sanitizeTransportError(platformErr.Cause)
	}
	return err
}

func queryWithFields(query url.Values, options ...socialhub.CallOption) (url.Values, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if !validFields(resolved.Fields) {
		return nil, invalidArgument("fields", "field selection is invalid")
	}
	if query == nil {
		query = url.Values{}
	}
	if len(resolved.Fields) > 0 {
		query.Set("fields", strings.Join(resolved.Fields, ","))
	}
	return query, nil
}

func pageQuery(cursor string, limit int) url.Values {
	query := url.Values{}
	if cursor != "" {
		query.Set("offset", cursor)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	return query
}

func nodes[T any](input []nodeEnvelope[T]) []T {
	output := make([]T, len(input))
	for index := range input {
		output[index] = input[index].Node
	}
	return output
}

func toPage[T any](items []T, source paging) (socialhub.Page[T], error) {
	result := socialhub.Page[T]{Items: items, HasMore: source.Next != ""}
	if source.Next != "" {
		cursor, err := pagingCursor(source.Next)
		if err != nil {
			return socialhub.Page[T]{}, platformError("paging", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		result.NextCursor = &cursor
	}
	if source.Previous != "" {
		cursor, err := pagingCursor(source.Previous)
		if err != nil {
			return socialhub.Page[T]{}, platformError("paging", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		result.PrevCursor = &cursor
	}
	return result, nil
}

func pagingCursor(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", &url.Error{Op: "parse", URL: "redacted paging URL", Err: errInvalidPaging}
	}
	cursor := parsed.Query().Get("offset")
	if cursor == "" || !validPage(cursor, 0) {
		return "", &url.Error{Op: "parse", URL: "redacted paging URL", Err: errInvalidPaging}
	}
	return cursor, nil
}

type pagingError string

func (e pagingError) Error() string { return string(e) }

const errInvalidPaging pagingError = "invalid offset cursor"

func escaped(value string) string { return url.PathEscape(value) }

var _ transport.ErrorDecoder = decodeHTTPError
