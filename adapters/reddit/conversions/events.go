package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

const submitEventsOperation = "conversion.submit_events"
const acceptedMessage = "Reddit accepted the conversion events"

func (client *Client) SubmitEvents(ctx context.Context, input SubmitEventsRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requireScope(submitEventsOperation); err != nil {
		return SubmitResult{}, err
	}
	prepared, err := prepareCallOptions(submitEventsOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	payload, err := normalizeRequest(input, client.clock.Now())
	if err != nil {
		return SubmitResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(submitEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(
		ctx, http.MethodPost, "/pixels/"+client.pixelID+"/conversion_events", nil, bytes.NewReader(encoded), prepared...,
	)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitResult{}, withRetrySafety(withOperation(err, submitEventsOperation), batchHasStableConversionIDs(input.Events))
	}
	return SubmitResult{StatusCode: metadata.StatusCode, Message: acceptedMessage}, nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(operation, "Reddit Conversions API does not document caller-supplied request IDs")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(operation, "Reddit Conversions API uses conversion_id for deduplication and does not document idempotency headers")
	}
	if len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "Reddit Conversions API does not support field selection")
	}
	if resolved.Timeout < 0 {
		return nil, invalidArgument(operation, "call timeout must not be negative")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func batchHasStableConversionIDs(events []ConversionEvent) bool {
	for _, event := range events {
		if event.Metadata == nil || event.Metadata.ConversionID == "" {
			return false
		}
	}
	return len(events) > 0
}
