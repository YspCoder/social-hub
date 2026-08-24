package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

const submitEventsOperation = "conversion.submit_events"

func (client *Client) SubmitEvents(ctx context.Context, input SubmitEventsRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requireScopes(submitEventsOperation); err != nil {
		return SubmitResult{}, err
	}
	events, err := normalizeRequest(client.conversionURN, client.clock.Now(), input)
	if err != nil {
		return SubmitResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	batch := len(events) > 1
	var payload any = events[0]
	if batch {
		payload = wireBatch{Elements: events}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(submitEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/conversionEvents", nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	if batch {
		request.Header.Set("X-RestLi-Method", "BATCH_CREATE")
	}
	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	if metadata.StatusCode != http.StatusCreated {
		return SubmitResult{}, platformContractError(submitEventsOperation, "LinkedIn returned a non-201 success response")
	}
	return SubmitResult{StatusCode: metadata.StatusCode, EventsAccepted: len(events), Batch: batch}, nil
}
