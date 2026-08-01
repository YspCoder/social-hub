package flickr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dghubble/oauth1"

	"social-hub/pkg/socialhub"
)

const maxAPIResponseSize int64 = 8 << 20

func (c *Client) call(ctx context.Context, httpMethod, apiMethod string, parameters url.Values, authenticated bool, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if resolved.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, resolved.Timeout)
		defer cancel()
	}
	values := cloneValues(parameters)
	values.Set("method", apiMethod)
	values.Set("api_key", c.apiKey)
	values.Set("format", "json")
	values.Set("nojsoncallback", "1")
	var request *http.Request
	if httpMethod == http.MethodGet {
		endpoint, parseErr := url.Parse(c.baseURL)
		if parseErr != nil {
			return platformError(apiMethod, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, parseErr)
		}
		endpoint.RawQuery = values.Encode()
		request, err = http.NewRequestWithContext(ctx, httpMethod, endpoint.String(), nil)
	} else {
		request, err = http.NewRequestWithContext(ctx, httpMethod, c.baseURL, strings.NewReader(values.Encode()))
		if err == nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return platformError(apiMethod, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	if resolved.RequestID != "" {
		request.Header.Set("X-Request-ID", resolved.RequestID)
	}
	client := c.public
	if authenticated {
		if c.signed == nil {
			return platformError(apiMethod, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, nil)
		}
		client = c.signed
	}
	response, err := client.Do(request)
	if err != nil {
		return platformError(apiMethod, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAPIResponseSize+1))
	if err != nil {
		return platformError(apiMethod, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxAPIResponseSize {
		return platformError(apiMethod, socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("Flickr response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeAPIError(apiMethod, response.StatusCode, response.Header, body)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return platformError(apiMethod, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if envelope.Stat != "ok" {
		return decodeAPIError(apiMethod, response.StatusCode, response.Header, body)
	}
	if output != nil {
		if err := json.Unmarshal(body, output); err != nil {
			return platformError(apiMethod, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	return nil
}

func signedHTTPClient(base *http.Client, config *oauth1.Config, token *oauth1.Token) *http.Client {
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	baseForOAuth := &http.Client{Transport: transport}
	ctx := context.WithValue(context.Background(), oauth1.HTTPClient, baseForOAuth)
	signed := config.Client(ctx, token)
	signed.Timeout = base.Timeout
	signed.Jar = base.Jar
	signed.CheckRedirect = rejectRedirect
	return signed
}

func noRedirectClient(base *http.Client) *http.Client {
	clone := *base
	clone.CheckRedirect = rejectRedirect
	return &clone
}

func rejectRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

func cloneValues(input url.Values) url.Values {
	output := make(url.Values, len(input)+4)
	for key, values := range input {
		output[key] = append([]string(nil), values...)
	}
	return output
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type capturedResponseTransport struct {
	ctx     context.Context
	base    http.RoundTripper
	body    []byte
	status  int
	header  http.Header
	maximum int64
}

func (transport *capturedResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request.Clone(transport.ctx))
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, transport.maximum+1))
	_ = response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if int64(len(body)) > transport.maximum {
		return nil, fmt.Errorf("OAuth response exceeded size limit")
	}
	transport.body = append([]byte(nil), body...)
	transport.status = response.StatusCode
	transport.header = response.Header.Clone()
	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, nil
}
