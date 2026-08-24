package panglemanagement

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maximumRequestBytes  = 1 << 20
	maximumResponseBytes = int64(8 << 20)
)

type apiEnvelope struct {
	Code      json.RawMessage `json:"code"`
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
}

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	path string,
	input any,
	signature string,
	mutation bool,
	options ...socialhub.CallOption,
) (apiEnvelope, int, http.Header, error) {
	callOptions, err := validateCallOptions(operation, options)
	if err != nil {
		return apiEnvelope{}, 0, nil, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return apiEnvelope{}, 0, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maximumRequestBytes {
		return apiEnvelope{}, 0, nil, invalidArgument(operation, "request JSON exceeds 1 MiB")
	}
	requestURL := *client.baseURL
	requestURL.Path = strings.TrimRight(client.baseURL.Path, "/") + path
	requestURL.RawPath = ""
	requestURL.RawQuery = ""
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(encoded))
	if err != nil {
		return apiEnvelope{}, 0, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	if client.sandbox {
		request.Header.Set("X-Tt-Env", "open_api_sandbox")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		failure := platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		return apiEnvelope{}, 0, nil, withMutationOutcome(operation, mutation, 0, failure)
	}
	defer response.Body.Close()
	redactions := []string{client.securityKey, string(client.userID), string(client.roleID), signature}
	safeHeader := sanitizedResponseHeaders(response.Header, redactions...)
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumResponseBytes+1))
	if err != nil {
		failure := platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
		return apiEnvelope{}, response.StatusCode, safeHeader,
			withMutationOutcome(operation, mutation, response.StatusCode, failure)
	}
	if int64(len(body)) > maximumResponseBytes {
		failure := platformContractError(operation, "Pangle response exceeded the 8 MiB size limit", response.StatusCode)
		return apiEnvelope{}, response.StatusCode, safeHeader,
			withMutationOutcome(operation, mutation, response.StatusCode, failure)
	}
	var envelope apiEnvelope
	if response.StatusCode != http.StatusOK {
		if json.Unmarshal(body, &envelope) != nil {
			envelope = apiEnvelope{}
		}
		failure := httpStatusError(
			operation, response.StatusCode, safeHeader, scalarCode(envelope.Code),
			safeRequestID(envelope.RequestID, redactions...), client.clock.Now(),
		)
		return apiEnvelope{}, response.StatusCode, safeHeader,
			withMutationOutcome(operation, mutation, response.StatusCode, failure)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		failure := platformContractError(operation, "Pangle returned an unexpected content type", response.StatusCode)
		return apiEnvelope{}, response.StatusCode, safeHeader, withMutationOutcome(operation, mutation, response.StatusCode, failure)
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		failure := platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		return apiEnvelope{}, response.StatusCode, safeHeader, withMutationOutcome(operation, mutation, response.StatusCode, failure)
	}
	envelope.RequestID = safeRequestID(envelope.RequestID, redactions...)
	return envelope, response.StatusCode, safeHeader, nil
}

func scalarCode(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var value string
		if json.Unmarshal(trimmed, &value) != nil || !validBusinessCode(value) {
			return ""
		}
		return value
	}
	if len(trimmed) > 20 {
		return ""
	}
	for _, character := range trimmed {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return string(trimmed)
}

func validBusinessCode(value string) bool {
	if value == "PG0000" {
		return true
	}
	if value == "" || len(value) > 20 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func requireData(operation string, envelope apiEnvelope, status int, output any) error {
	trimmed := bytes.TrimSpace(envelope.Data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return platformContractError(operation, "Pangle success response omitted data", status)
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}
