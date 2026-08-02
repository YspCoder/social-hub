package lastfm

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) get(ctx context.Context, method string, parameters url.Values, signed bool, output any, options ...socialhub.CallOption) error {
	values := cloneValues(parameters)
	values.Set("method", method)
	values.Set("api_key", c.apiKey)
	if signed {
		if err := c.requireSecret(method); err != nil {
			return err
		}
		values.Set("api_sig", signature(values, c.apiSecret))
	}
	values.Set("format", "json")
	request, err := c.api.NewRequest(ctx, http.MethodGet, "", values, nil, options...)
	if err != nil {
		return err
	}
	return c.execute(request, method, output)
}

func (c *Client) post(ctx context.Context, method string, parameters url.Values, output any, options ...socialhub.CallOption) error {
	if err := c.requireSession(method); err != nil {
		return err
	}
	values := cloneValues(parameters)
	values.Set("method", method)
	values.Set("api_key", c.apiKey)
	values.Set("sk", c.sessionKey)
	values.Set("api_sig", signature(values, c.apiSecret))
	values.Set("format", "json")
	request, err := c.api.NewRequest(ctx, http.MethodPost, "", nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.execute(request, method, output)
}

func (c *Client) execute(request *http.Request, operation string, output any) error {
	var raw json.RawMessage
	metadata, err := c.api.DoWithMetadata(request, &raw)
	if err != nil {
		if platformErr, ok := err.(*socialhub.Error); ok {
			platformErr.Op = operation
		}
		return err
	}
	var envelope apiErrorEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if envelope.Error != nil {
		return lastFMError(operation, metadata.StatusCode, *envelope.Error, envelope.Message, metadata.Header)
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func signature(parameters url.Values, secret string) string {
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		if key != "format" && key != "callback" && key != "api_sig" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var input strings.Builder
	for _, key := range keys {
		for _, value := range parameters[key] {
			input.WriteString(key)
			input.WriteString(value)
		}
	}
	input.WriteString(secret)
	digest := md5.Sum([]byte(input.String())) // Last.fm's published signature algorithm requires MD5.
	return hex.EncodeToString(digest[:])
}

func cloneValues(input url.Values) url.Values {
	output := make(url.Values, len(input))
	for key, values := range input {
		output[key] = append([]string(nil), values...)
	}
	return output
}
