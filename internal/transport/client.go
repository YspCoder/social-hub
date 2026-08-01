// Package transport provides the shared authenticated HTTP transport used by
// platform adapters.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const defaultMaxResponseBytes int64 = 8 << 20

type cancelContextKey struct{}

// ErrorDecoder maps an unsuccessful platform response to a socialhub error.
type ErrorDecoder func(status int, header http.Header, body []byte) error

// Client executes bounded, authenticated platform requests.
type Client struct {
	baseURL          *url.URL
	httpClient       *http.Client
	tokens           socialhub.TokenSource
	platform         socialhub.Platform
	product          string
	maxResponseBytes int64
	decodeError      ErrorDecoder
}

// New creates a shared transport client.
func New(baseURL string, httpClient *http.Client, tokens socialhub.TokenSource, platform socialhub.Platform, product string, decodeError ErrorDecoder) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("transport: invalid base URL")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("transport: HTTP client is required")
	}
	if tokens == nil {
		return nil, fmt.Errorf("transport: token source is required")
	}
	return &Client{
		baseURL:          parsed,
		httpClient:       httpClient,
		tokens:           tokens,
		platform:         platform,
		product:          product,
		maxResponseBytes: defaultMaxResponseBytes,
		decodeError:      decodeError,
	}, nil
}

// JSON sends a JSON request and decodes a JSON response.
func (c *Client) JSON(ctx context.Context, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Op: method + " " + path, Platform: string(c.platform), Cause: err}
		}
		body = bytes.NewReader(encoded)
	}
	request, err := c.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return c.Do(request, output)
}

// NewRequest creates an authenticated request. The path must be relative to the
// configured platform origin.
func (c *Client) NewRequest(ctx context.Context, method, path string, query url.Values, body io.Reader, options ...socialhub.CallOption) (*http.Request, error) {
	if strings.Contains(path, "://") {
		return nil, &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: string(c.platform), Op: method, PlatformMessage: "request path must be relative"}
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	requestURL := *c.baseURL
	requestURL.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	requestURL.RawQuery = query.Encode()
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		ctx = context.WithValue(ctx, cancelContextKey{}, cancel)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: string(c.platform), Op: method + " " + path, Cause: err}
	}
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, &socialhub.Error{Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: string(c.platform), Product: c.product, Op: method + " " + path, Cause: err}
	}
	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	request.Header.Set("Authorization", tokenType+" "+token.AccessToken)
	request.Header.Set("Accept", "application/json")
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	if callOptions.IdempotencyKey != "" {
		request.Header.Set("Idempotency-Key", callOptions.IdempotencyKey)
	}
	return request, nil
}

// Do executes a prepared request and decodes its bounded response.
func (c *Client) Do(request *http.Request, output any) error {
	if cancel, ok := request.Context().Value(cancelContextKey{}).(context.CancelFunc); ok {
		defer cancel()
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return &socialhub.Error{Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable, Platform: string(c.platform), Product: c.product, Op: request.Method + " " + request.URL.Path, Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, c.maxResponseBytes+1))
	if err != nil {
		return &socialhub.Error{Code: socialhub.CodeTemporarilyUnavailable, Class: socialhub.ClassRetryable, Platform: string(c.platform), Product: c.product, Op: request.Method + " " + request.URL.Path, HTTPStatus: response.StatusCode, Cause: err}
	}
	if int64(len(body)) > c.maxResponseBytes {
		return &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: string(c.platform), Product: c.product, Op: request.Method + " " + request.URL.Path, HTTPStatus: response.StatusCode, PlatformMessage: "response exceeded size limit"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if c.decodeError != nil {
			return c.decodeError(response.StatusCode, response.Header, body)
		}
		return defaultHTTPError(c.platform, c.product, request.Method+" "+request.URL.Path, response.StatusCode, response.Header)
	}
	if output == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, output); err != nil {
		return &socialhub.Error{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Platform: string(c.platform), Product: c.product, Op: request.Method + " " + request.URL.Path, HTTPStatus: response.StatusCode, Cause: err}
	}
	return nil
}

func defaultHTTPError(platform socialhub.Platform, product, operation string, status int, header http.Header) error {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound:
		code = socialhub.CodeNotFound
	case http.StatusConflict:
		code = socialhub.CodeConflict
	case http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
	}
	return &socialhub.Error{
		Code:       code,
		Class:      class,
		Platform:   string(platform),
		Product:    product,
		Op:         operation,
		HTTPStatus: status,
		RequestID:  firstHeader(header, "x-request-id", "x-correlation-id"),
	}
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}
