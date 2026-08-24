package jinghongads

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

type apiEnvelope struct {
	Code json.RawMessage `json:"code"`
	Data json.RawMessage `json:"data"`
}

func (client *Client) doJSON(ctx context.Context, operation, scope, method, path string, input, output any, options ...socialhub.CallOption) error {
	if err := client.requireScope(operation, scope); err != nil {
		return err
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return invalidArgument(operation, "request JSON exceeds 1 MiB")
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(ctx, method, path, nil, body, forwardCallOptions(callOptions)...)
	if err != nil {
		return withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	var envelope apiEnvelope
	metadata, err := client.api.DoWithMetadata(request, &envelope)
	if err != nil {
		return withOperation(err, operation)
	}
	code := scalarCode(envelope.Code)
	if code != "0" && code != "200" {
		if code == "" {
			return platformContractError(operation, "Jinghong response omitted a valid business code")
		}
		return businessError(operation, metadata.StatusCode, metadata.Header, code, client.clock.Now())
	}
	if output == nil {
		return nil
	}
	if len(envelope.Data) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return platformContractError(operation, "Jinghong success response omitted data")
	}
	if err := json.Unmarshal(envelope.Data, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}
