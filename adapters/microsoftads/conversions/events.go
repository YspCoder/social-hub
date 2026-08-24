package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

const submitEventsOperation = "conversion.submit_events"

type submitResponse struct {
	EventsReceived *int `json:"eventsReceived"`
	Error          *struct {
		Details []struct {
			IsWarning bool `json:"isWarning"`
		} `json:"details"`
	} `json:"error"`
}

func (client *Client) SubmitEvents(ctx context.Context, input SubmitEventsRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	if callOptions.RequestID != "" {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "Microsoft CAPI does not define caller request IDs")
	}
	if callOptions.IdempotencyKey != "" {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "Microsoft CAPI does not define an idempotency-key header; use a stable eventId per event")
	}
	if len(callOptions.Fields) != 0 {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "conversion submission does not support response field selection")
	}
	if callOptions.Timeout < 0 {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "call timeout must not be negative")
	}
	payload, err := normalizeRequest(client.clock.Now(), input)
	if err != nil {
		return SubmitResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(submitEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	requestOptions := []socialhub.CallOption(nil)
	if callOptions.Timeout > 0 {
		requestOptions = append(requestOptions, socialhub.WithCallTimeout(callOptions.Timeout))
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/v1/"+client.tagID+"/events", nil, bytes.NewReader(encoded), requestOptions...)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	var response submitResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withRetrySafety(withOperation(err, submitEventsOperation), batchHasStableEventIDs(input.Events))
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(submitEventsOperation, "Microsoft returned a non-200 success response")
	}
	if response.EventsReceived != nil && *response.EventsReceived != len(input.Events) {
		return SubmitResult{}, platformContractError(submitEventsOperation, "Microsoft returned an inconsistent eventsReceived count")
	}
	hasWarnings := false
	if response.Error != nil {
		for _, detail := range response.Error.Details {
			if !detail.IsWarning {
				return SubmitResult{}, platformContractError(submitEventsOperation, "Microsoft returned an error diagnostic with HTTP 200")
			}
			hasWarnings = true
		}
	}
	return SubmitResult{StatusCode: metadata.StatusCode, EventsAccepted: len(input.Events), HasWarnings: hasWarnings}, nil
}

func batchHasStableEventIDs(events []ConversionEvent) bool {
	for _, event := range events {
		if event.EventID == "" {
			return false
		}
	}
	return len(events) > 0
}
