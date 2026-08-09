package admob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	maxErrorResponseBytes  int64 = 1 << 20
	maxReportResponseBytes int64 = 512 << 20
)

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodGet, path, query, nil, output, options...), operation)
}

func (client *Client) postReport(ctx context.Context, operation, path string, input any, expected reportExpectation, options ...socialhub.CallOption) (*Report, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, invalidArgument(operation, "report request cannot be encoded")
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(body))
	if err != nil {
		return nil, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	if callOptions.IdempotencyKey != "" {
		request.Header.Set("Idempotency-Key", callOptions.IdempotencyKey)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeCause(err))
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes+1))
		if readErr != nil {
			return nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(payload)) > maxErrorResponseBytes {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("error response exceeded size limit"))
		}
		return nil, withOperation(decodeHTTPError(response.StatusCode, response.Header, payload), operation)
	}
	limited := &io.LimitedReader{R: response.Body, N: maxReportResponseBytes + 1}
	report, err := decodeReport(limited, expected)
	if limited.N == 0 {
		return nil, platformContractError(operation, "AdMob report exceeded response size limit")
	}
	if err != nil {
		return nil, platformContractError(operation, "AdMob returned a malformed report: "+boundedMessage(err.Error(), 256))
	}
	return report, nil
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}
