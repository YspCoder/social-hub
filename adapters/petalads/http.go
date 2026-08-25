package petalads

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

type apiEnvelope struct {
	Code json.RawMessage `json:"code"`
	Data json.RawMessage `json:"data"`
}

func (client *Client) doJSON(ctx context.Context, operation, scope, method, path string, input any, output any, options ...socialhub.CallOption) error {
	if err := client.requireScope(operation, scope); err != nil {
		return err
	}
	safeOptions, err := prepareCallOptions(operation, options)
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
	request, err := client.api.NewRequest(ctx, method, path, nil, body, safeOptions...)
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
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return platformContractError(operation, "Petal Ads success response was not application/json")
	}
	code := numericCode(envelope.Code)
	if code != "0" && code != "200" {
		if code == "" {
			return platformContractError(operation, "Petal Ads response omitted a valid business code")
		}
		return businessError(operation, metadata.StatusCode, metadata.Header, code, client.clock.Now(), client.requestIDValues...)
	}
	if output == nil {
		return nil
	}
	if len(envelope.Data) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return platformContractError(operation, "Petal Ads success response omitted data")
	}
	if err := json.Unmarshal(envelope.Data, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}
