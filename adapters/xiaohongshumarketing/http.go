package xiaohongshumarketing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

type apiEnvelope struct {
	Code           json.RawMessage `json:"code"`
	ErrorCode      json.RawMessage `json:"errorCode"`
	ErrorCodeSnake json.RawMessage `json:"error_code"`
	ErrCode        json.RawMessage `json:"errcode"`
	Success        *bool           `json:"success"`
	Data           json.RawMessage `json:"data"`
	RequestID      string          `json:"request_id"`
}

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	path string,
	input any,
	mutation bool,
	options ...socialhub.CallOption,
) (json.RawMessage, string, error) {
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
		err = withOperation(err, operation)
		if mutation && ambiguousMutationError(err) {
			return nil, "", outcomeUnknownError(operation, err)
		}
		return nil, "", err
	}
	if len(raw) == 0 || !json.Valid(raw) {
		if mutation {
			return nil, "", outcomeUnknownError(operation, platformContractError(operation, "Spotlight returned an invalid JSON response"))
		}
		return nil, "", platformContractError(operation, "Spotlight returned an invalid JSON response")
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		if mutation {
			return nil, "", outcomeUnknownError(operation, err)
		}
		return nil, "", platformContractError(operation, "Spotlight returned an invalid response envelope")
	}
	requestID := safeOpaque(firstNonEmpty(envelope.RequestID, firstHeader(metadata.Header, "X-Request-ID", "X-Trace-ID")), 256)
	code := firstNonEmpty(
		scalarCode(envelope.ErrorCode), scalarCode(envelope.ErrorCodeSnake),
		scalarCode(envelope.ErrCode), scalarCode(envelope.Code),
	)
	if envelope.Success != nil && !*envelope.Success {
		return nil, "", businessError(operation, metadata.StatusCode, metadata.Header, code, requestID)
	}
	if code != "" && code != "0" && code != "200" {
		return nil, "", businessError(operation, metadata.StatusCode, metadata.Header, code, requestID)
	}
	if envelope.Success == nil && code == "" {
		if mutation {
			return nil, "", outcomeUnknownError(operation, platformContractError(operation, "Spotlight response omitted success and business code"))
		}
		return nil, "", platformContractError(operation, "Spotlight response omitted success and business code")
	}
	return append(json.RawMessage(nil), envelope.Data...), requestID, nil
}

func decodeRequiredData(operation string, raw json.RawMessage, output any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return platformContractError(operation, "Spotlight success response omitted data")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return platformContractError(operation, "Spotlight returned invalid response data")
	}
	return nil
}

func ambiguousMutationError(err error) bool {
	var hub *socialhub.Error
	if !errors.As(err, &hub) {
		return false
	}
	if hub.HTTPStatus == 0 {
		return hub.Code == socialhub.CodeTemporarilyUnavailable || hub.Code == socialhub.CodePlatformError
	}
	return hub.Code == socialhub.CodeTemporarilyUnavailable || hub.HTTPStatus >= 200 && hub.HTTPStatus < 300
}
