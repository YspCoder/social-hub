package tenjin

import (
	"bytes"
	"context"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	trackOpenOperation         = "s2s.track_open"
	trackCustomEventOperation  = "s2s.track_custom_event"
	trackPurchaseOperation     = "s2s.track_purchase"
	trackAdImpressionOperation = "s2s.track_ad_impression"
	maximumRequestBytes        = 1 << 20
)

type ingestionResponse struct {
	Code int `json:"code"`
}

func (client *Client) TrackOpen(ctx context.Context, input OpenRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	requestOptions, err := prepareCallOptions(trackOpenOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateOpen(client, input); err != nil {
		return SubmitResult{}, invalidArgument(trackOpenOperation, err.Error())
	}
	return client.submitAcknowledged(ctx, trackOpenOperation, "/v0/event", client.openForm(input), requestOptions...)
}

func (client *Client) TrackCustomEvent(ctx context.Context, input CustomEventRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	requestOptions, err := prepareCallOptions(trackCustomEventOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateCustomEvent(client, input); err != nil {
		return SubmitResult{}, invalidArgument(trackCustomEventOperation, err.Error())
	}
	return client.submitAcknowledged(ctx, trackCustomEventOperation, "/v0/event", client.customEventForm(input), requestOptions...)
}

func (client *Client) TrackPurchase(ctx context.Context, input PurchaseRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	requestOptions, err := prepareCallOptions(trackPurchaseOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validatePurchase(client, input); err != nil {
		return SubmitResult{}, invalidArgument(trackPurchaseOperation, err.Error())
	}
	return client.submitAcknowledged(ctx, trackPurchaseOperation, "/v0/purchase", client.purchaseForm(input), requestOptions...)
}

func (client *Client) TrackAdImpression(ctx context.Context, input AdImpressionRequest, options ...socialhub.CallOption) (SubmitResult, error) {
	requestOptions, err := prepareCallOptions(trackAdImpressionOperation, options)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := validateAdImpression(client, input); err != nil {
		return SubmitResult{}, invalidArgument(trackAdImpressionOperation, err.Error())
	}
	return client.submitHTTPAccepted(ctx, trackAdImpressionOperation, "/v0/ad_impressions/", client.adImpressionForm(input), requestOptions...)
}

func (client *Client) submitAcknowledged(ctx context.Context, operation, path string, form url.Values, options ...socialhub.CallOption) (SubmitResult, error) {
	request, err := client.formRequest(ctx, path, form, options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	var response ingestionResponse
	metadata, err := client.api.DoWithMetadata(request, &response)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(operation, "Tenjin returned an undocumented success status", metadata.StatusCode)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return SubmitResult{}, platformContractError(operation, "Tenjin returned a non-JSON success response", metadata.StatusCode)
	}
	if response.Code != http.StatusOK {
		return SubmitResult{}, responseCodeError(operation, metadata.StatusCode, response.Code)
	}
	return SubmitResult{StatusCode: metadata.StatusCode}, nil
}

func (client *Client) submitHTTPAccepted(ctx context.Context, operation, path string, form url.Values, options ...socialhub.CallOption) (SubmitResult, error) {
	request, err := client.formRequest(ctx, path, form, options...)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	metadata, err := client.api.DoWithMetadata(request, nil)
	if err != nil {
		return SubmitResult{}, withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return SubmitResult{}, platformContractError(operation, "Tenjin returned an undocumented success status", metadata.StatusCode)
	}
	return SubmitResult{StatusCode: metadata.StatusCode}, nil
}

func (client *Client) formRequest(ctx context.Context, path string, form url.Values, options ...socialhub.CallOption) (*http.Request, error) {
	encoded := form.Encode()
	if len(encoded) > maximumRequestBytes {
		return nil, invalidArgument(http.MethodPost+" "+path, "encoded request exceeds the adapter's 1 MiB safety limit")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewBufferString(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request, nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, withOperation(err, operation)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "only per-call timeouts are supported by Tenjin S2S submission")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
