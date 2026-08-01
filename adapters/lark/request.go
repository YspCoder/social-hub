package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type apiEnvelope struct {
	Code  *int            `json:"code"`
	Msg   string          `json:"msg"`
	Error json.RawMessage `json:"error"`
}

func (c *Client) call(ctx context.Context, operation, method, path string, query url.Values, input, output any, allowUUID bool, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return unsupported(operation, "field selection is not supported by this Open Platform operation")
	}
	if resolved.IdempotencyKey != "" && !allowUUID {
		return unsupported(operation, "this Open Platform operation does not document a UUID idempotency field")
	}
	var body bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = *bytes.NewReader(encoded)
	}
	request, err := c.api.NewRequest(ctx, method, path, query, &body, cleanCallOptions(resolved)...)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	request.Header.Del("Idempotency-Key")
	var raw json.RawMessage
	metadata, err := c.api.DoWithMetadata(request, &raw)
	if err != nil {
		return operationError(err, operation)
	}
	var envelope apiEnvelope
	if len(raw) == 0 || json.Unmarshal(raw, &envelope) != nil || envelope.Code == nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if *envelope.Code != 0 {
		return apiResponseError(operation, metadata.StatusCode, metadata.Header, envelope)
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func cleanCallOptions(resolved socialhub.CallOptions) []socialhub.CallOption {
	options := make([]socialhub.CallOption, 0, 2)
	if resolved.RequestID != "" {
		options = append(options, socialhub.WithRequestID(resolved.RequestID))
	}
	if resolved.Timeout > 0 {
		options = append(options, socialhub.WithCallTimeout(resolved.Timeout))
	}
	return options
}

func operationError(err error, operation string) error {
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return err
}

func (c *Client) get(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return c.call(ctx, operation, http.MethodGet, path, query, nil, output, false, options...)
}
