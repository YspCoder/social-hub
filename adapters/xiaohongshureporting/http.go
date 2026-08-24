package xiaohongshureporting

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

const (
	maxRequestBytes     = 1 << 20
	maxReportValueBytes = 1 << 20
)

type apiEnvelope struct {
	Code           json.RawMessage `json:"code"`
	ErrorCode      json.RawMessage `json:"errorCode"`
	ErrorCodeSnake json.RawMessage `json:"error_code"`
	ErrCode        json.RawMessage `json:"errcode"`
	Success        *bool           `json:"success"`
	RequestID      string          `json:"request_id"`
}

func (client *Client) doJSON(ctx context.Context, operation, path string, input any, options ...socialhub.CallOption) (json.RawMessage, string, error) {
	if err := validateCallOptions(operation, options); err != nil {
		return nil, "", err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, "", platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maxRequestBytes {
		return nil, "", invalidArgument(operation, "request JSON exceeds 1 MiB")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, "", withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return nil, "", withOperation(err, operation)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, "", platformContractError(operation, "Spotlight returned an invalid JSON response")
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, "", platformContractError(operation, "Spotlight returned an invalid response envelope")
	}
	requestID := safeOpaque(firstNonEmpty(envelope.RequestID, firstHeader(metadata.Header, "X-Request-ID", "X-Trace-ID")), 256)
	code := firstNonEmpty(
		scalarCode(envelope.ErrorCode), scalarCode(envelope.ErrorCodeSnake),
		scalarCode(envelope.ErrCode), scalarCode(envelope.Code),
	)
	if envelope.Success != nil {
		if !*envelope.Success {
			return nil, "", businessError(operation, metadata.StatusCode, metadata.Header, code, requestID)
		}
	} else if code != "0" && code != "200" {
		if code == "" {
			return nil, "", platformContractError(operation, "Spotlight response omitted success and business code")
		}
		return nil, "", businessError(operation, metadata.StatusCode, metadata.Header, code, requestID)
	}
	return append(json.RawMessage(nil), raw...), requestID, nil
}

func rawObjectField(raw json.RawMessage, key string) (json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, false
	}
	value, found := object[key]
	return value, found
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
