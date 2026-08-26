package quoraconversions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	submitEventOperation     = "conversion.submit_event"
	submitEventsOperation    = "conversion.submit_events"
	maximumRequestBytes      = 8 << 20
	maximumResponseTextRunes = 4096
)

type wireSingleRequest struct {
	AccountID  int64      `json:"account_id"`
	Debug      bool       `json:"debug,omitempty"`
	User       User       `json:"user"`
	Device     Device     `json:"device"`
	Conversion Conversion `json:"conversion"`
}

type wireSingleResponse struct {
	Status   EventStatus   `json:"status"`
	Warnings []wireWarning `json:"warnings"`
}

type wireWarning struct {
	Code    WarningCode `json:"code"`
	Message string      `json:"message"`
}

type wireBatchRequest struct {
	AccountID int64             `json:"account_id"`
	Debug     bool              `json:"debug,omitempty"`
	Data      []ConversionEvent `json:"data"`
}

type wireBatchResponse struct {
	EventsReceived int               `json:"events_received"`
	EventsErrored  int               `json:"events_errored"`
	Events         []wireEventResult `json:"events"`
}

type wireEventResult struct {
	Status       EventStatus   `json:"status"`
	Index        int           `json:"index"`
	ErrorCode    string        `json:"error_code"`
	ErrorMessage string        `json:"error_message"`
	Warnings     []wireWarning `json:"warnings"`
}

func (client *Client) SubmitEvent(ctx context.Context, input SubmitEventRequest, options ...socialhub.CallOption) (SubmitEventResult, error) {
	requestOptions, err := prepareCallOptions(submitEventOperation, options)
	if err != nil {
		return SubmitEventResult{}, err
	}
	if err := validateEvent(input.Event); err != nil {
		return SubmitEventResult{}, invalidArgument(submitEventOperation, err.Error())
	}
	payload, err := json.Marshal(wireSingleRequest{
		AccountID: client.adAccountID, Debug: input.Debug,
		User: input.Event.User, Device: input.Event.Device, Conversion: input.Event.Conversion,
	})
	if err != nil {
		return SubmitEventResult{}, platformError(submitEventOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(payload) > maximumRequestBytes {
		return SubmitEventResult{}, invalidArgument(submitEventOperation, "encoded request exceeds the 8 MiB adapter limit")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/conversion", nil, bytes.NewReader(payload), requestOptions...)
	if err != nil {
		return SubmitEventResult{}, withOperation(err, submitEventOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	if input.Debug {
		request.Header.Set("Accept", "application/json")
	} else {
		request.Header.Set("Accept", "text/plain")
	}

	result := SubmitEventResult{Status: EventStatusOK}
	if input.Debug {
		var response wireSingleResponse
		metadata, err := client.api.DoWithMetadata(request, &response)
		if err != nil {
			return SubmitEventResult{}, withRetrySafety(withOperation(err, submitEventOperation), eventHasStableID(input.Event))
		}
		result.StatusCode = metadata.StatusCode
		if metadata.StatusCode != http.StatusOK {
			return SubmitEventResult{}, platformContractError(submitEventOperation, "Quora returned a non-200 success response")
		}
		if !hasMediaType(metadata.Header.Get("Content-Type"), "application/json") {
			return SubmitEventResult{}, platformContractError(submitEventOperation, "Quora returned a non-JSON debug response")
		}
		if response.Status != EventStatusOK {
			return SubmitEventResult{}, platformContractError(submitEventOperation, "Quora returned an invalid debug response")
		}
		warnings, err := validateWarnings(response.Warnings)
		if err != nil {
			return SubmitEventResult{}, platformContractError(submitEventOperation, err.Error())
		}
		result.Warnings = warnings
		return result, nil
	}

	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitEventResult{}, withRetrySafety(withOperation(err, submitEventOperation), eventHasStableID(input.Event))
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitEventResult{}, platformContractError(submitEventOperation, "Quora returned a non-200 success response")
	}
	if !hasMediaType(metadata.Header.Get("Content-Type"), "text/plain") {
		return SubmitEventResult{}, platformContractError(submitEventOperation, "Quora returned a non-text success response")
	}
	result.StatusCode = metadata.StatusCode
	return result, nil
}

func (client *Client) SubmitEvents(ctx context.Context, input SubmitEventsRequest, options ...socialhub.CallOption) (SubmitEventsResult, error) {
	requestOptions, err := prepareCallOptions(submitEventsOperation, options)
	if err != nil {
		return SubmitEventsResult{}, err
	}
	if err := validateBatch(input.Events); err != nil {
		return SubmitEventsResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	payload, err := json.Marshal(wireBatchRequest{AccountID: client.adAccountID, Debug: input.Debug, Data: input.Events})
	if err != nil {
		return SubmitEventsResult{}, platformError(submitEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(payload) > maximumRequestBytes {
		return SubmitEventsResult{}, invalidArgument(submitEventsOperation, "encoded request exceeds the 8 MiB adapter limit")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/conversions", nil, bytes.NewReader(payload), requestOptions...)
	if err != nil {
		return SubmitEventsResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	var response wireBatchResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitEventsResult{}, withRetrySafety(withOperation(err, submitEventsOperation), batchHasStableEventIDs(input.Events))
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitEventsResult{}, platformContractError(submitEventsOperation, "Quora returned a non-200 success response")
	}
	if !hasMediaType(metadata.Header.Get("Content-Type"), "application/json") {
		return SubmitEventsResult{}, platformContractError(submitEventsOperation, "Quora returned a non-JSON batch response")
	}
	events, err := validateBatchResponse(response, len(input.Events), input.Debug)
	if err != nil {
		return SubmitEventsResult{}, platformContractError(submitEventsOperation, err.Error())
	}
	return SubmitEventsResult{
		StatusCode: metadata.StatusCode, EventsReceived: response.EventsReceived,
		EventsErrored: response.EventsErrored, Events: events,
	}, nil
}

func validateBatchResponse(response wireBatchResponse, requested int, debug bool) ([]EventResult, error) {
	if response.EventsReceived != requested || response.EventsErrored < 0 ||
		response.EventsErrored > response.EventsReceived || len(response.Events) != requested {
		return nil, fmt.Errorf("Quora returned inconsistent event counts")
	}
	errored := 0
	results := make([]EventResult, len(response.Events))
	for index, event := range response.Events {
		if event.Index != index {
			return nil, fmt.Errorf("Quora returned an out-of-order event index")
		}
		if !validErrorCode(event.ErrorCode) || !validResponseText(event.ErrorMessage, maximumResponseTextRunes) {
			return nil, fmt.Errorf("Quora returned an invalid event diagnostic")
		}
		warnings, err := validateWarnings(event.Warnings)
		if err != nil {
			return nil, err
		}
		if !debug && len(warnings) != 0 {
			return nil, fmt.Errorf("Quora returned debug warnings for a non-debug batch")
		}
		switch event.Status {
		case EventStatusOK:
			if event.ErrorCode != "" || event.ErrorMessage != "" {
				return nil, fmt.Errorf("Quora returned an error diagnostic for a successful event")
			}
		case EventStatusError:
			errored++
		default:
			return nil, fmt.Errorf("Quora returned an invalid event status")
		}
		results[index] = EventResult{
			Status: event.Status, Index: event.Index, ErrorCode: event.ErrorCode,
			HasErrorMessage: event.ErrorMessage != "", Warnings: warnings,
		}
	}
	if errored != response.EventsErrored {
		return nil, fmt.Errorf("Quora returned an error count that does not match event statuses")
	}
	return results, nil
}

func validateWarnings(warnings []wireWarning) ([]Warning, error) {
	if len(warnings) > 3 {
		return nil, fmt.Errorf("Quora returned too many warning diagnostics")
	}
	seen := make(map[WarningCode]struct{}, len(warnings))
	result := make([]Warning, len(warnings))
	for index, warning := range warnings {
		if !validWarningCode(warning.Code) || !validResponseText(warning.Message, maximumResponseTextRunes) {
			return nil, fmt.Errorf("Quora returned an invalid warning diagnostic")
		}
		if _, duplicate := seen[warning.Code]; duplicate {
			return nil, fmt.Errorf("Quora returned a duplicate warning code")
		}
		seen[warning.Code] = struct{}{}
		result[index] = Warning{Code: warning.Code}
	}
	return result, nil
}

func eventHasStableID(event ConversionEvent) bool {
	return event.Conversion.EventID != ""
}

func batchHasStableEventIDs(events []ConversionEvent) bool {
	for _, event := range events {
		if !eventHasStableID(event) {
			return false
		}
	}
	return len(events) != 0
}

func hasMediaType(value, expected string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, expected)
}
