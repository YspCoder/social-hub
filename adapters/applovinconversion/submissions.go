package applovinconversion

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	submitEventsOperation = "conversion.submit_events"
	maximumRequestBytes   = 1 << 20
)

type eventEnvelope struct {
	Events []ServerEvent `json:"events"`
}

func (client *Client) SubmitEvents(ctx context.Context, events []ServerEvent, options ...socialhub.CallOption) (SubmitResult, error) {
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	if callOptions.RequestID != "" {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "AppLovin Conversion API does not define caller request IDs")
	}
	if callOptions.IdempotencyKey != "" {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "AppLovin Conversion API does not define an idempotency-key header; use a stable dedupe_id per event")
	}
	if len(callOptions.Fields) != 0 {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "Conversion API event submission does not support response field selection")
	}
	if callOptions.Timeout < 0 {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "call timeout must not be negative")
	}
	if err := validateBatch(client.policy, events); err != nil {
		return SubmitResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	var wirePayload any = events
	if client.policy != PolicyStandard {
		wirePayload = eventEnvelope{Events: events}
	}
	payload, err := json.Marshal(wirePayload)
	if err != nil {
		return SubmitResult{}, platformError(submitEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(payload) > maximumRequestBytes {
		return SubmitResult{}, invalidArgument(submitEventsOperation, "encoded request exceeds the adapter's 1 MiB safety limit")
	}
	query := url.Values{"pixel_id": []string{client.eventKey}}
	requestOptions := []socialhub.CallOption(nil)
	if callOptions.Timeout > 0 {
		requestOptions = append(requestOptions, socialhub.WithCallTimeout(callOptions.Timeout))
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/event", query, bytes.NewReader(payload), requestOptions...)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitResult{}, withRetrySafety(withOperation(err, submitEventsOperation), batchHasStableDedupeIDs(events))
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(submitEventsOperation, "AppLovin Conversion API returned a non-200 success response")
	}
	return SubmitResult{StatusCode: metadata.StatusCode, EventCount: len(events)}, nil
}

func batchHasStableDedupeIDs(events []ServerEvent) bool {
	for _, event := range events {
		if event.DedupeID == "" {
			return false
		}
	}
	return len(events) > 0
}
