package micropub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	maxResponseBytes int64 = 8 << 20
	maxTokenBytes          = 8 << 10
)

type response struct {
	Status int
	Header http.Header
	Body   []byte
}

func (client *Client) endpointRequest(ctx context.Context, method string, query url.Values, body io.Reader, options ...socialhub.CallOption) (*http.Request, context.CancelFunc, error) {
	return client.newRequest(ctx, method, client.endpoint, query, body, options...)
}

func (client *Client) newRequest(ctx context.Context, method string, endpoint *url.URL, query url.Values, body io.Reader, options ...socialhub.CallOption) (*http.Request, context.CancelFunc, error) {
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, nil, err
	}
	requestURL := *endpoint
	merged := requestURL.Query()
	for key, values := range query {
		merged.Del(key)
		for _, value := range values {
			merged.Add(key, value)
		}
	}
	requestURL.RawQuery = merged.Encode()
	cancel := func() {}
	if callOptions.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		cancel()
		return nil, nil, invalidArgument(method, "unable to construct request")
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("Accept", "application/json")
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	if callOptions.IdempotencyKey != "" {
		request.Header.Set("Idempotency-Key", callOptions.IdempotencyKey)
	}
	return request, cancel, nil
}

func (client *Client) do(request *http.Request, cancel context.CancelFunc) (response, error) {
	defer cancel()
	httpResponse, err := client.httpClient.Do(request)
	if err != nil {
		var urlError *url.Error
		if errors.As(err, &urlError) && urlError.Err != nil {
			err = urlError.Err
		}
		return response{}, platformError(request.Method+" "+request.URL.Path, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer httpResponse.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxResponseBytes+1))
	result := response{Status: httpResponse.StatusCode, Header: httpResponse.Header.Clone(), Body: body}
	if err != nil {
		return result, platformError(request.Method+" "+request.URL.Path, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxResponseBytes {
		return result, &socialhub.Error{
			Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
			Platform: platformName, Product: productName, Op: request.Method + " " + request.URL.Path,
			HTTPStatus: httpResponse.StatusCode, PlatformMessage: "response exceeded size limit",
		}
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return result, decodeHTTPError(httpResponse.StatusCode, httpResponse.Header, body, request.Method+" "+request.URL.Path)
	}
	return result, nil
}

func (client *Client) jsonRequest(ctx context.Context, method string, payload, output any, options ...socialhub.CallOption) (response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return response{}, invalidArgument(method, "request contains invalid JSON values")
	}
	request, cancel, err := client.endpointRequest(ctx, method, nil, bytes.NewReader(body), options...)
	if err != nil {
		return response{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	result, err := client.do(request, cancel)
	if err != nil {
		return result, err
	}
	if output != nil && len(result.Body) != 0 {
		if err := json.Unmarshal(result.Body, output); err != nil {
			return result, platformError(method, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return result, nil
}

func (client *Client) queryJSON(ctx context.Context, query url.Values, output any, options ...socialhub.CallOption) (response, error) {
	request, cancel, err := client.endpointRequest(ctx, http.MethodGet, query, nil, options...)
	if err != nil {
		return response{}, err
	}
	result, err := client.do(request, cancel)
	if err != nil {
		return result, err
	}
	if output != nil {
		if len(result.Body) == 0 || json.Unmarshal(result.Body, output) != nil {
			return result, platformError("query", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
	}
	return result, nil
}
