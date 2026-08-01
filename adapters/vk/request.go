package vk

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type apiEnvelope struct {
	Response json.RawMessage `json:"response"`
	Error    *apiError       `json:"error"`
}

func (c *Client) method(ctx context.Context, method string, values url.Values, output any, options ...socialhub.CallOption) error {
	if values == nil {
		values = make(url.Values)
	}
	values.Set("v", apiVersion)
	request, err := c.api.NewRequest(ctx, http.MethodPost, method, nil, bytes.NewBufferString(values.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Del("Idempotency-Key")
	var envelope apiEnvelope
	if err := c.api.Do(request, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return envelope.Error.err(method)
	}
	if len(envelope.Response) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Response), []byte("null")) {
		return platformError(method, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Response, output); err != nil {
		return platformError(method, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}
