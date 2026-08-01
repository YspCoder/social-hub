package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

type apiEnvelope struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Warning  string `json:"warning"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
	Metadata struct {
		Messages []string `json:"messages"`
		Warnings []string `json:"warnings"`
	} `json:"response_metadata"`
}

func (c *Client) call(ctx context.Context, method string, input, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if resolved.IdempotencyKey != "" {
		return unsupported(method, "Slack does not document idempotency keys for this Web API method")
	}
	body := []byte("{}")
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(method, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = encoded
	}
	request, err := c.api.NewRequest(ctx, http.MethodPost, method, nil, bytes.NewReader(body), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Del("Idempotency-Key")
	var raw json.RawMessage
	if err := c.api.Do(request, &raw); err != nil {
		return err
	}
	var envelope apiEnvelope
	if len(raw) == 0 || json.Unmarshal(raw, &envelope) != nil {
		return platformError(method, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if !envelope.OK {
		return apiResponseError(method, envelope)
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return platformError(method, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}
