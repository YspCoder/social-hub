package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	submitEventsOperation    = "conversion.submit_events"
	validateEventsOperation  = "conversion.validate_events"
	validationLogsOperation  = "conversion.validation_logs"
	validationStatsOperation = "conversion.validation_stats"
	maximumRequestBytes      = 8 << 20
	acceptedReason           = "Snap accepted the conversion events"
)

type statusResponse struct {
	Status    string               `json:"status"`
	Reason    string               `json:"reason"`
	TestEvent bool                 `json:"test_event"`
	EventLogs []ValidationEventLog `json:"event_logs"`
}

func (client *Client) SubmitEvents(ctx context.Context, events []ServerEvent, options ...socialhub.CallOption) (SubmitResult, error) {
	requestOptions, err := conversionCallOptions(submitEventsOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	payload, err := normalizeBatch(client.assetType, events, client.clock.Now())
	if err != nil {
		return SubmitResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	var response statusResponse
	statusCode, err := client.postEvents(
		ctx, "/"+client.assetID+"/events", payload, &response, submitEventsOperation,
		batchHasStableEventIDs(events), requestOptions,
	)
	if err != nil {
		return SubmitResult{}, err
	}
	if response.Status != "VALID" || !validResponseText(response.Reason, 4096) {
		return SubmitResult{}, platformContractError(submitEventsOperation, "Snap did not confirm event ingestion")
	}
	return SubmitResult{StatusCode: statusCode, Status: response.Status, Reason: acceptedReason}, nil
}

func (client *Client) ValidateEvents(ctx context.Context, events []ServerEvent, options ...socialhub.CallOption) (ValidationResult, error) {
	requestOptions, err := conversionCallOptions(validateEventsOperation, options)
	if err != nil {
		return ValidationResult{}, err
	}
	payload, err := normalizeBatch(client.assetType, events, client.clock.Now())
	if err != nil {
		return ValidationResult{}, invalidArgument(validateEventsOperation, err.Error())
	}
	var response statusResponse
	statusCode, err := client.postEvents(
		ctx, "/"+client.assetID+"/events/validate", payload, &response, validateEventsOperation, true, requestOptions,
	)
	if err != nil {
		return ValidationResult{}, err
	}
	if response.Status != "VALID" || !response.TestEvent || !validResponseText(response.Reason, 4096) ||
		!validValidationEventLogs(response.EventLogs, len(events)) {
		return ValidationResult{}, platformContractError(validateEventsOperation, "Snap returned an invalid validation response")
	}
	return ValidationResult{
		StatusCode: statusCode, Status: response.Status, TestEvent: response.TestEvent,
		Reason: response.Reason, EventLogs: append([]ValidationEventLog(nil), response.EventLogs...),
	}, nil
}

func (client *Client) postEvents(
	ctx context.Context,
	path string,
	payload wireEnvelope,
	output any,
	operation string,
	retrySafe bool,
	options []socialhub.CallOption,
) (int, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maximumRequestBytes {
		return 0, invalidArgument(operation, "encoded request exceeds the adapter's 8 MiB safety limit")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return 0, withRetrySafety(withOperation(err, operation), retrySafe)
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.api.DoWithMetadata(request, output)
	if err != nil {
		return metadata.StatusCode, withRetrySafety(withOperation(err, operation), retrySafe)
	}
	if metadata.StatusCode != http.StatusOK {
		return metadata.StatusCode, platformContractError(operation, "Snap returned a non-200 success response")
	}
	return metadata.StatusCode, nil
}

func (client *Client) GetValidationLogs(ctx context.Context, options ...socialhub.CallOption) (ValidationLogsResult, error) {
	requestOptions, err := conversionCallOptions(validationLogsOperation, options)
	if err != nil {
		return ValidationLogsResult{}, err
	}
	var response ValidationLogsResult
	request, err := client.api.NewRequest(
		ctx, http.MethodGet, "/"+client.assetID+"/events/validate/logs", nil, nil, requestOptions...,
	)
	if err != nil {
		return ValidationLogsResult{}, withOperation(err, validationLogsOperation)
	}
	if err := client.api.Do(request, &response); err != nil {
		return ValidationLogsResult{}, withOperation(err, validationLogsOperation)
	}
	if response.Status != "SUCCESS" || !validResponseText(response.Reason, 4096) || !validValidationLogs(response.Logs) {
		return ValidationLogsResult{}, platformContractError(validationLogsOperation, "Snap returned an invalid validation logs response")
	}
	return response, nil
}

func (client *Client) GetValidationStats(ctx context.Context, options ...socialhub.CallOption) (ValidationStatsResult, error) {
	requestOptions, err := conversionCallOptions(validationStatsOperation, options)
	if err != nil {
		return ValidationStatsResult{}, err
	}
	var response ValidationStatsResult
	request, err := client.api.NewRequest(
		ctx, http.MethodGet, "/"+client.assetID+"/events/validate/stats", nil, nil, requestOptions...,
	)
	if err != nil {
		return ValidationStatsResult{}, withOperation(err, validationStatsOperation)
	}
	if err := client.api.Do(request, &response); err != nil {
		return ValidationStatsResult{}, withOperation(err, validationStatsOperation)
	}
	if response.Status != "SUCCESS" || !validResponseText(response.Reason, 4096) ||
		response.Stats.Test.LatestEventTimestamp < 0 || response.Stats.Test.EventCountPastHour < 0 {
		return ValidationStatsResult{}, platformContractError(validationStatsOperation, "Snap returned an invalid validation stats response")
	}
	return response, nil
}

func validValidationEventLogs(logs []ValidationEventLog, eventCount int) bool {
	if len(logs) > eventCount {
		return false
	}
	for _, log := range logs {
		if log.Event < 1 || log.Event > eventCount || !validResponseText(log.Status, 128) ||
			!validResponseStrings(log.Errors.Codes, 256) || !validResponseStrings(log.Errors.Messages, 4096) {
			return false
		}
	}
	return true
}

func validValidationLogs(logs []ValidationLog) bool {
	if len(logs) > 10_000 {
		return false
	}
	for _, log := range logs {
		for _, field := range []string{log.EventName, log.EventTime, log.ActionSource, log.Status, log.AssetID, log.RawEventName} {
			if !validResponseText(field, 4096) {
				return false
			}
		}
		if !validResponseStrings(log.ErrorRecords, 4096) || !validResponseStrings(log.WarningRecords, 4096) {
			return false
		}
	}
	return true
}

func validResponseStrings(values []string, maximum int) bool {
	if len(values) > 1000 {
		return false
	}
	for _, value := range values {
		if !validResponseText(value, maximum) {
			return false
		}
	}
	return true
}

func validResponseText(value string, maximum int) bool {
	return len(value) <= maximum && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func conversionCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Snap Conversions API does not define caller request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "Snap Conversions API does not define an idempotency-key header; use a stable event_id per event")
	}
	if len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "Snap Conversions API does not support response field selection")
	}
	if resolved.Timeout < 0 {
		return nil, invalidArgument(operation, "call timeout must not be negative")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func batchHasStableEventIDs(events []ServerEvent) bool {
	for _, event := range events {
		if event.EventID == "" {
			return false
		}
	}
	return len(events) > 0
}
