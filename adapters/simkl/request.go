package simkl

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func requestJSON(ctx context.Context, client *transport.Client, operation, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	if len(resolved.Fields) > 0 {
		return transport.ResponseMetadata{}, invalidArgument(operation, "field selection is fixed by the typed Simkl operation")
	}
	if resolved.IdempotencyKey != "" {
		return transport.ResponseMetadata{}, invalidArgument(operation, "Simkl does not document an idempotency-key contract")
	}
	body := bytes.NewReader(nil)
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return transport.ResponseMetadata{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	metadata, err := client.DoWithMetadata(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return metadata, err
}

func pageFromMetadata[T any](items []T, requestedPage int, metadata transport.ResponseMetadata) socialhub.Page[T] {
	page := headerInt(metadata.Header, "X-Pagination-Page")
	pageCount := headerInt(metadata.Header, "X-Pagination-Page-Count")
	if page == 0 {
		page = requestedPage
		if page == 0 {
			page = 1
		}
	}
	result := socialhub.Page[T]{Items: items, HasMore: pageCount > 0 && page < pageCount}
	if result.HasMore {
		next := strconv.Itoa(page + 1)
		result.NextCursor = &next
	}
	if page > 1 {
		previous := strconv.Itoa(page - 1)
		result.PrevCursor = &previous
	}
	return result
}

func headerInt(header http.Header, name string) int {
	value, err := strconv.Atoi(strings.TrimSpace(header.Get(name)))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func escaped(value string) string { return url.PathEscape(value) }
