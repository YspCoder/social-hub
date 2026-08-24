package ads

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

func (client *Client) accountPath(resource string) string {
	return "/ad_accounts/" + client.adAccountID + "/" + resource
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireScopes(operation, readScope); err != nil {
		return transport.ResponseMetadata{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	metadata, err := client.api.DoWithMetadata(request, output)
	client.recordRateLimit(metadata.Header)
	return metadata, err
}

func (client *Client) writeJSON(ctx context.Context, operation, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireScopes(operation, editScope); err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.jsonRequest(ctx, method, path, query, input, output, options...)
}

func (client *Client) reportJSON(ctx context.Context, operation, path string, query url.Values, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireScopes(operation, readScope); err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.jsonRequest(ctx, http.MethodPost, path, query, input, output, options...)
}

func (client *Client) jsonRequest(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return transport.ResponseMetadata{}, platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, bytes.NewReader(encoded), options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.api.DoWithMetadata(request, output)
	client.recordRateLimit(metadata.Header)
	return metadata, err
}

func listQuery(input ListRequest) url.Values {
	query := make(url.Values)
	if input.Cursor != "" {
		query.Set("page.token", input.Cursor)
	}
	if input.PageSize > 0 {
		query.Set("page.size", strconv.Itoa(input.PageSize))
	}
	return query
}

func (client *Client) pageCursor(operation, requestPath string, nextURL *string) (*string, error) {
	if nextURL == nil || *nextURL == "" {
		return nil, nil
	}
	if len(*nextURL) > 32768 {
		return nil, platformContractError(operation, "Reddit returned an oversized pagination URL")
	}
	parsed, err := url.Parse(*nextURL)
	if err != nil || parsed.User != nil || parsed.Fragment != "" ||
		!strings.EqualFold(parsed.Scheme, client.baseURL.Scheme) || !strings.EqualFold(parsed.Host, client.baseURL.Host) {
		return nil, platformContractError(operation, "Reddit returned an invalid pagination origin")
	}
	expectedPath := strings.TrimRight(client.baseURL.Path, "/") + "/" + strings.TrimLeft(requestPath, "/")
	if parsed.Path != expectedPath {
		return nil, platformContractError(operation, "Reddit returned an invalid pagination path")
	}
	values := parsed.Query()["page.token"]
	if len(values) != 1 || !validOpaque(values[0], 16384) {
		return nil, platformContractError(operation, "Reddit returned an invalid pagination token")
	}
	cursor := values[0]
	return &cursor, nil
}

func page[T any](items []T, cursor *string) socialhub.Page[T] {
	return socialhub.Page[T]{Items: items, NextCursor: cursor, HasMore: cursor != nil}
}

func formatInt(value int) string { return strconv.Itoa(value) }
