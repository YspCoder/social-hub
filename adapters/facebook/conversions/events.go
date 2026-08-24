package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const submitEventsOperation = "conversion.submit_events"

type eventResponse struct {
	EventsReceived      *int     `json:"events_received"`
	Messages            []string `json:"messages"`
	TraceID             string   `json:"fbtrace_id"`
	DatasetID           string   `json:"id"`
	NumProcessedEntries *int     `json:"num_processed_entries"`
}

func (client *Client) SubmitEvents(ctx context.Context, input SubmitEventsRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requireScope(submitEventsOperation); err != nil {
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
		ctx, http.MethodPost, "/"+client.pixelID+"/events", nil, bytes.NewReader(encoded), options...,
	)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	var response eventResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(submitEventsOperation, "Meta returned a non-200 success response")
	}
	if !isJSONContentType(metadata.Header.Get("Content-Type")) {
		return SubmitResult{}, platformContractError(submitEventsOperation, "Meta returned a non-JSON success response")
	}
	if err := validateEventResponse(response, len(input.Events), client.pixelID); err != nil {
		return SubmitResult{}, platformContractError(submitEventsOperation, err.Error())
	}
	return SubmitResult{
		StatusCode: metadata.StatusCode, EventsReceived: response.EventsReceived,
		MessageCount: len(response.Messages), TraceID: response.TraceID,
		DatasetID: response.DatasetID, NumProcessedEntries: response.NumProcessedEntries,
	}, nil
}

func validateEventResponse(response eventResponse, requested int, expectedDatasetID string) error {
	for _, counter := range []struct {
		name  string
		value *int
	}{
		{"events_received", response.EventsReceived},
		{"num_processed_entries", response.NumProcessedEntries},
	} {
		if counter.value != nil && (*counter.value < 0 || *counter.value > requested) {
			return fmt.Errorf("Meta returned an invalid %s count", counter.name)
		}
	}
	if len(response.Messages) > 1000 {
		return fmt.Errorf("Meta returned too many response messages")
	}
	for _, message := range response.Messages {
		if len(message) > 4096 || !utf8.ValidString(message) {
			return fmt.Errorf("Meta returned an invalid response message")
		}
	}
	if !validOptionalOpaque(response.TraceID, 256) || !validOptionalOpaque(response.DatasetID, 256) {
		return fmt.Errorf("Meta returned invalid response metadata")
	}
	if response.DatasetID != "" && response.DatasetID != expectedDatasetID {
		return fmt.Errorf("Meta returned an unexpected dataset ID")
	}
	return nil
}

func isJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}
