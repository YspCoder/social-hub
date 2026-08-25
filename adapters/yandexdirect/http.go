package yandexdirect

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

type rpcEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *apiError       `json:"error"`
}

type rpcRequest struct {
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

func (client *Client) rpc(
	ctx context.Context,
	operation, service, method string,
	params, output any,
	mutation bool,
	options ...socialhub.CallOption,
) (ResponseMetadata, error) {
	if err := client.requireAccess(operation); err != nil {
		return ResponseMetadata{}, err
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMetadata{}, err
	}
	encoded, err := json.Marshal(rpcRequest{Method: method, Params: params})
	if err != nil {
		return ResponseMetadata{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maxRequestBytes {
		return ResponseMetadata{}, invalidArgument(operation, "request JSON exceeds 1 MiB")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, service, nil, bytes.NewReader(encoded), forwardCallOptions(callOptions)...)
	if err != nil {
		if mutation && ambiguousMutationError(err) {
			return ResponseMetadata{}, outcomeUnknownError(operation, err, ResponseMetadata{})
		}
		return ResponseMetadata{}, withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept-Encoding", "identity")
	client.applyHeaders(request)
	var envelope rpcEnvelope
	wireMetadata, err := client.api.DoWithMetadata(request, &envelope)
	metadata := responseMetadata(wireMetadata.Header, client.requestIDValues...)
	if err != nil {
		err = withRequestMetadata(err, operation, metadata)
		if mutation && ambiguousMutationError(err) {
			return metadata, outcomeUnknownError(operation, err, metadata)
		}
		return metadata, err
	}
	mediaType, _, contentTypeErr := mime.ParseMediaType(wireMetadata.Header.Get("Content-Type"))
	if contentTypeErr != nil || mediaType != "application/json" {
		contractErr := platformContractError(operation, "Yandex returned a non-JSON successful response")
		if mutation {
			return metadata, outcomeUnknownError(operation, contractErr, metadata)
		}
		return metadata, contractErr
	}
	if envelope.Error != nil {
		return metadata, apiErrorValue(
			operation, wireMetadata.StatusCode, wireMetadata.Header, *envelope.Error, client.clock.Now(), client.requestIDValues...,
		)
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		err := platformContractError(operation, "Yandex returned neither result nor error")
		if mutation {
			return metadata, outcomeUnknownError(operation, err, metadata)
		}
		return metadata, err
	}
	if output != nil {
		if err := json.Unmarshal(envelope.Result, output); err != nil {
			contractErr := platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
			if mutation {
				return metadata, outcomeUnknownError(operation, contractErr, metadata)
			}
			return metadata, contractErr
		}
	}
	return metadata, nil
}

func responseMetadata(header http.Header, requestIDValues ...string) ResponseMetadata {
	return ResponseMetadata{
		RequestID:      responseRequestID(requestIDValues, header.Get("RequestId")),
		Units:          parseUnits(header.Get("Units")),
		UnitsUsedLogin: boundedLogin(header.Get("Units-Used-Login")),
	}
}

func parseUnits(value string) *Units {
	value = boundedOpaque(value, 128)
	if value == "" {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 3 {
		return nil
	}
	values := [3]int64{}
	for index, part := range parts {
		parsed, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || parsed < 0 {
			return nil
		}
		values[index] = parsed
	}
	return &Units{Spent: values[0], Remaining: values[1], DailyLimit: values[2]}
}

func actionResult(operation string, items []ActionResult, expected int, metadata ResponseMetadata, expectedIDs ...int64) (BatchResult, error) {
	result := BatchResult{Items: items, Metadata: metadata}
	if len(items) != expected || len(expectedIDs) != 0 && len(expectedIDs) != expected {
		return result, outcomeUnknownError(operation, platformContractError(operation, "Yandex returned an unexpected number of per-item results"), metadata)
	}
	seenIDs := make(map[int64]struct{}, len(items))
	for index := range result.Items {
		item := &result.Items[index]
		if item.ID < 0 || item.ID == 0 && len(item.Errors) == 0 || item.ID > 0 && len(item.Errors) > 0 {
			return result, outcomeUnknownError(operation, platformContractError(operation, "Yandex returned an invalid per-item mutation result"), metadata)
		}
		if item.ID > 0 {
			if _, duplicate := seenIDs[item.ID]; duplicate || len(expectedIDs) > 0 && item.ID != expectedIDs[index] {
				return result, outcomeUnknownError(operation, platformContractError(operation, "Yandex returned a duplicate or mismatched mutation result ID"), metadata)
			}
			seenIDs[item.ID] = struct{}{}
		}
		if len(item.Warnings) > 100 || len(item.Errors) > 100 {
			return result, outcomeUnknownError(operation, platformContractError(operation, "Yandex returned too many mutation diagnostics"), metadata)
		}
		for notificationIndex := range item.Warnings {
			if !sanitizeNotification(&item.Warnings[notificationIndex]) {
				return result, outcomeUnknownError(operation, platformContractError(operation, "Yandex returned invalid mutation diagnostics"), metadata)
			}
		}
		for notificationIndex := range item.Errors {
			if !sanitizeNotification(&item.Errors[notificationIndex]) {
				return result, outcomeUnknownError(operation, platformContractError(operation, "Yandex returned invalid mutation diagnostics"), metadata)
			}
		}
	}
	return result, batchResultError(operation, result)
}

func sanitizeNotification(notification *Notification) bool {
	if notification == nil || notification.Code <= 0 {
		return false
	}
	message := boundedSingleLine(redactSensitive(notification.Message), 2048)
	details := boundedSingleLine(redactSensitive(notification.Details), 4096)
	if message == "" || notification.Details != "" && details == "" {
		return false
	}
	notification.Message, notification.Details = message, details
	return true
}
