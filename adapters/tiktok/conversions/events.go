package conversions

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const submitEventsOperation = "conversion.submit_events"

type submitEnvelope struct {
	Code      *int64          `json:"code"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
}

func (client *Client) SubmitEvents(ctx context.Context, input SubmitEventsRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	if err := client.requirePermission(submitEventsOperation); err != nil {
		return SubmitResult{}, err
	}
	payload, err := normalizeRequest(client.eventSource, client.eventSourceID, input)
	if err != nil {
		return SubmitResult{}, invalidArgument(submitEventsOperation, err.Error())
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SubmitResult{}, platformError(submitEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/event/track/", nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	request.Header.Set("Content-Type", "application/json")
	var response submitEnvelope
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withOperation(err, submitEventsOperation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(submitEventsOperation, "TikTok returned a non-200 success response")
	}
	if !isJSONContentType(metadata.Header.Get("Content-Type")) {
		return SubmitResult{}, platformContractError(submitEventsOperation, "TikTok returned a non-JSON success response")
	}
	requestID, err := client.validateSubmitResponse(response, metadata.Header)
	if err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{
		StatusCode: metadata.StatusCode, RequestID: requestID, EventsAccepted: len(input.Events),
	}, nil
}

func isJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func (client *Client) validateSubmitResponse(response submitEnvelope, header http.Header) (string, error) {
	if response.Code == nil {
		return "", platformContractError(submitEventsOperation, "TikTok response omitted code")
	}
	if *response.Code != 0 {
		return "", conversionError(submitEventsOperation, http.StatusOK, response.Code, response.RequestID, header, client.clock.Now())
	}
	requestID := firstNonEmpty(response.RequestID, header.Get("x-request-id"), header.Get("x-tt-logid"))
	if !validOpaque(requestID, 256) {
		return "", platformContractError(submitEventsOperation, "TikTok response omitted a valid request_id")
	}
	var data map[string]json.RawMessage
	if len(response.Data) == 0 || json.Unmarshal(response.Data, &data) != nil || data == nil {
		return "", platformContractError(submitEventsOperation, "TikTok success response omitted object data")
	}
	return requestID, nil
}
