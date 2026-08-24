package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const submitEventsOperation = "conversion.submit_events"

type wireSubmitResponse struct {
	Events             []wireEventResponse `json:"events"`
	NumEventsProcessed int                 `json:"num_events_processed"`
	NumEventsReceived  int                 `json:"num_events_received"`
}

type wireEventResponse struct {
	Status         EventStatus `json:"status"`
	ErrorMessage   *string     `json:"error_message"`
	WarningMessage *string     `json:"warning_message"`
}

func (client *Client) SubmitEvents(ctx context.Context, input SubmitEventsRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requireScope(submitEventsOperation); err != nil {
		return SubmitResult{}, err
	}
	call, err := validateCallOptions(submitEventsOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	payload, err := normalizeRequest(input)
	if err != nil {
		return SubmitResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(submitEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	query := url.Values{}
	if input.Test {
		query.Set("test", "true")
	}
	request, err := client.api.NewRequest(
		ctx, http.MethodPost, "/ad_accounts/"+client.adAccountID+"/events", query, bytes.NewReader(encoded), resolvedCallOptions(call)...,
	)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	var response wireSubmitResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withOperation(protectOfflineRetry(err, input.Events), submitEventsOperation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(submitEventsOperation, "Pinterest returned a non-200 success response")
	}
	if !isJSONContentType(metadata.Header.Get("Content-Type")) {
		return SubmitResult{}, platformContractError(submitEventsOperation, "Pinterest returned a non-JSON success response")
	}
	results, err := validateSubmitResponse(response, len(input.Events))
	if err != nil {
		return SubmitResult{}, platformContractError(submitEventsOperation, err.Error())
	}
	return SubmitResult{
		StatusCode: metadata.StatusCode, EventsReceived: response.NumEventsReceived,
		EventsProcessed: response.NumEventsProcessed, Events: results,
	}, nil
}

func isJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func protectOfflineRetry(err error, events []ConversionEvent) error {
	if err == nil || !containsOfflineEvent(events) {
		return err
	}
	var hub *socialhub.Error
	if !errors.As(err, &hub) || !hub.Retryable() || hub.Code == socialhub.CodeRateLimited {
		return err
	}
	copy := *hub
	copy.Class = socialhub.ClassUserAction
	copy.PlatformMessage = "offline conversion submission outcome is ambiguous; do not retry automatically"
	return &copy
}

func containsOfflineEvent(events []ConversionEvent) bool {
	for _, event := range events {
		if event.ActionSource == ActionSourceOffline {
			return true
		}
	}
	return false
}

func validateSubmitResponse(response wireSubmitResponse, requested int) ([]EventResult, error) {
	if response.NumEventsReceived != requested || response.NumEventsProcessed < 0 ||
		response.NumEventsProcessed > response.NumEventsReceived || len(response.Events) != requested {
		return nil, fmt.Errorf("Pinterest returned inconsistent event counts")
	}
	processed := 0
	results := make([]EventResult, len(response.Events))
	for index, event := range response.Events {
		if event.Status != EventStatusProcessed && event.Status != EventStatusFailed {
			return nil, fmt.Errorf("Pinterest returned an invalid event status")
		}
		if event.ErrorMessage != nil && !validResponseText(*event.ErrorMessage, 4096) ||
			event.WarningMessage != nil && !validResponseText(*event.WarningMessage, 4096) {
			return nil, fmt.Errorf("Pinterest returned an invalid event message")
		}
		if event.Status == EventStatusProcessed {
			processed++
		}
		results[index] = EventResult{
			Index: index, Status: event.Status,
			HasError:   event.ErrorMessage != nil && *event.ErrorMessage != "",
			HasWarning: event.WarningMessage != nil && *event.WarningMessage != "",
		}
	}
	if processed != response.NumEventsProcessed {
		return nil, fmt.Errorf("Pinterest returned a processed count that does not match event statuses")
	}
	return results, nil
}
