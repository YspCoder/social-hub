package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) json(ctx context.Context, method, escapedPath string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(method+" "+escapedPath, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.newRequest(ctx, method, escapedPath, query, body, options...)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return client.api.Do(request, output)
}

func (client *Client) newRequest(ctx context.Context, method, escapedPath string, query url.Values, body io.Reader, options ...socialhub.CallOption) (*http.Request, error) {
	if !strings.HasPrefix(escapedPath, "/_matrix/") || strings.ContainsAny(escapedPath, "?#") {
		return nil, invalidArgument("request", "Matrix request path is invalid")
	}
	request, err := client.api.NewRequest(ctx, method, "/_matrix/placeholder", query, body, options...)
	if err != nil {
		return nil, err
	}
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return nil, invalidArgument("request", "Matrix request path encoding is invalid")
	}
	request.URL.Path = decodedPath
	request.URL.RawPath = escapedPath
	return request, nil
}

func matrixPath(prefix string, segments ...string) string {
	path := strings.TrimRight(prefix, "/")
	for _, segment := range segments {
		path += "/" + url.PathEscape(segment)
	}
	return path
}

func transactionID(options ...socialhub.CallOption) (string, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return "", err
	}
	if resolved.IdempotencyKey != "" {
		if !validOpaque(resolved.IdempotencyKey, 1024) {
			return "", invalidArgument("transaction_id", "idempotency key is not a valid Matrix transaction ID")
		}
		return resolved.IdempotencyKey, nil
	}
	value, err := randomTransactionID()
	if err != nil {
		return "", platformError("transaction_id", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return value, nil
}
