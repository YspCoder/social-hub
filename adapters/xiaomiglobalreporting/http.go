package xiaomiglobalreporting

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

type apiEnvelope struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
	TraceID string          `json:"traceId"`
}

type timestampSequence struct {
	mu   sync.Mutex
	last int64
	unit TimestampUnit
}

func (sequence *timestampSequence) next(clock socialhub.Clock) (int64, error) {
	if sequence == nil || clock == nil {
		return 0, platformContractError("request_metadata", "timestamp generator is unavailable")
	}
	now := clock.Now()
	var current int64
	switch sequence.unit {
	case TimestampUnixSeconds:
		current = now.Unix()
	case TimestampUnixMilliseconds:
		current = now.UnixMilli()
	default:
		return 0, platformContractError("request_metadata", "timestamp unit is invalid")
	}
	if current <= 0 {
		return 0, platformContractError("request_metadata", "clock returned a nonpositive timestamp")
	}
	sequence.mu.Lock()
	defer sequence.mu.Unlock()
	if current <= sequence.last {
		if sequence.last == 1<<63-1 {
			return 0, platformContractError("request_metadata", "timestamp sequence overflowed")
		}
		current = sequence.last + 1
	}
	sequence.last = current
	return current, nil
}

func newRequestUID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", platformError("request_metadata", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	return hex.EncodeToString(buffer), nil
}

func (client *Client) doJSON(ctx context.Context, operation, path string, input any, options ...socialhub.CallOption) (json.RawMessage, string, error) {
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return nil, "", err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, "", platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maxRequestBytes {
		return nil, "", invalidArgument(operation, "request JSON exceeds 1 MiB")
	}
	timestamp, err := client.timestamps.next(client.clock)
	if err != nil {
		return nil, "", withOperationAndRequestID(err, operation, "", client.redactionSecrets...)
	}
	requestUID, err := newRequestUID()
	if err != nil {
		return nil, "", withOperationAndRequestID(err, operation, "", client.redactionSecrets...)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), prepared...)
	if err != nil {
		return nil, "", withOperationAndRequestID(err, operation, requestUID, client.redactionSecrets...)
	}
	request.AddCookie(&http.Cookie{Name: "timestamp", Value: strconv.FormatInt(timestamp, 10)})
	request.AddCookie(&http.Cookie{Name: "uid", Value: requestUID})
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return nil, requestUID, withOperationAndRequestID(err, operation, requestUID, client.redactionSecrets...)
	}
	if metadata.StatusCode != http.StatusOK {
		return nil, requestUID, platformContractError(operation, "Xiaomi returned an unexpected successful HTTP status", metadata.StatusCode)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return nil, requestUID, platformContractError(operation, "Xiaomi returned a non-JSON response", metadata.StatusCode)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, requestUID, platformContractError(operation, "Xiaomi returned an invalid JSON response", metadata.StatusCode)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, requestUID, platformContractError(operation, "Xiaomi returned an invalid response envelope", metadata.StatusCode)
	}
	code := scalarCode(envelope.Code)
	if code == "" {
		return nil, requestUID, platformContractError(operation, "Xiaomi response omitted its business code", metadata.StatusCode)
	}
	if code != "0" {
		return nil, requestUID, businessError(
			operation, metadata.StatusCode, metadata.Header, code, envelope.Message,
			envelope.TraceID, requestUID, client.clock.Now(), client.redactionSecrets...,
		)
	}
	if len(envelope.Result) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Result), []byte("null")) {
		return nil, requestUID, platformContractError(operation, "Xiaomi success response omitted its result", metadata.StatusCode)
	}
	return append(json.RawMessage(nil), envelope.Result...), requestUID, nil
}
