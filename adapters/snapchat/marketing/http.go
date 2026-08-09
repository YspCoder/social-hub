package marketing

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

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireScope(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.api.DoWithMetadata(request, output)
}

func (client *Client) writeJSON(ctx context.Context, operation, method, path string, input, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	if err := client.requireScope(operation); err != nil {
		return transport.ResponseMetadata{}, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return transport.ResponseMetadata{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, method, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	contentType := "application/json"
	if method == http.MethodPatch {
		contentType = "application/json-patch+json"
	}
	request.Header.Set("Content-Type", contentType)
	return client.api.DoWithMetadata(request, output)
}

func listQuery(input ListRequest) url.Values {
	query := make(url.Values)
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	if input.Limit > 0 {
		query.Set("limit", fmtInt(input.Limit))
	}
	return query
}

func (client *Client) pageCursor(operation, requestPath, nextLink string) (*string, error) {
	if nextLink == "" {
		return nil, nil
	}
	parsed, err := url.Parse(nextLink)
	if err != nil || parsed.User != nil || parsed.Fragment != "" ||
		!strings.EqualFold(parsed.Scheme, client.baseURL.Scheme) || !strings.EqualFold(parsed.Host, client.baseURL.Host) {
		return nil, platformContractError(operation, "Snapchat returned an invalid pagination origin")
	}
	expectedPath := strings.TrimRight(client.baseURL.Path, "/") + "/" + strings.TrimLeft(requestPath, "/")
	if parsed.Path != expectedPath {
		return nil, platformContractError(operation, "Snapchat returned an invalid pagination path")
	}
	values := parsed.Query()["cursor"]
	if len(values) != 1 || !validOpaque(values[0], 16384) {
		return nil, platformContractError(operation, "Snapchat returned an invalid pagination cursor")
	}
	cursor := values[0]
	return &cursor, nil
}

func fmtInt(value int) string {
	return strconv.Itoa(value)
}
