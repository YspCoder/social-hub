package googledatamanager

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

const ingestEventsOperation = "events.ingest"

func (client *Client) IngestEvents(ctx context.Context, input IngestEventsRequest, options ...socialhub.CallOption) (*IngestEventsResponse, error) {
	if err := client.requireScope(ingestEventsOperation); err != nil {
		return nil, err
	}
	callOptions, err := supportedCallOptions(options)
	if err != nil {
		return nil, err
	}
	payload, err := normalizeRequest(input)
	if err != nil {
		return nil, invalidArgument(ingestEventsOperation, err.Error())
	}
	var response IngestEventsResponse
	if err := withOperation(client.api.JSON(ctx, http.MethodPost, "/v1/events:ingest", nil, payload, &response, callOptions...), ingestEventsOperation); err != nil {
		return nil, err
	}
	if !validOpaque(response.RequestID, 1024) {
		return nil, platformContractError(ingestEventsOperation, "Google returned an invalid requestId")
	}
	if len(response.FieldWarnings) > MaximumEventsPerRequest*MaximumUserIdentifiers {
		return nil, platformContractError(ingestEventsOperation, "Google returned too many field warnings")
	}
	for index := range response.FieldWarnings {
		response.FieldWarnings[index].Description = boundedMessage(redactSensitive(response.FieldWarnings[index].Description), 2048)
		response.FieldWarnings[index].Field = boundedMessage(redactSensitive(response.FieldWarnings[index].Field), 1024)
		if !validWarning(response.FieldWarnings[index]) {
			return nil, platformContractError(ingestEventsOperation, "Google returned an invalid field warning")
		}
	}
	return &response, nil
}

func supportedCallOptions(options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(ingestEventsOperation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return nil, invalidArgument(ingestEventsOperation, "Google assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return nil, invalidArgument(ingestEventsOperation, "events.ingest does not define a request-level idempotency key; use a stable event transactionId where supported")
	}
	if len(resolved.Fields) > 0 {
		return nil, invalidArgument(ingestEventsOperation, "events.ingest does not support response field selection")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func validWarning(value FieldWarning) bool {
	return validWarningReason(value.Reason) && validText(value.Description, 2048, false) && validText(value.Field, 1024, false)
}

func validWarningReason(value WarningReason) bool {
	switch value {
	case "", WarningCustomVariableNotEnabled, WarningCustomVariableNotPredefined,
		WarningCartDataNotSupportedWithBraid, WarningCartItemProductIDMissing,
		WarningCartItemUnitPriceMissing, WarningGeneric, WarningInvalidClientID,
		WarningInvalidSubdivisionCode, WarningInvalidRegionCode, WarningInvalidSubcontinentCode,
		WarningInvalidContinentCode, WarningInvalidDeviceCategory,
		WarningInvalidDeviceScreenResolution, WarningInvalidMerchantID:
		return true
	default:
		return false
	}
}
