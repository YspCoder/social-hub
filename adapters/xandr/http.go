package xandr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type apiEnvelope struct {
	Response *responseWire `json:"response"`
}

type responseWire struct {
	Status       string          `json:"status"`
	Token        string          `json:"token"`
	ErrorID      string          `json:"error_id"`
	Error        string          `json:"error"`
	Advertiser   json.RawMessage `json:"advertiser"`
	Advertisers  json.RawMessage `json:"advertisers"`
	Campaign     json.RawMessage `json:"campaign"`
	Campaigns    json.RawMessage `json:"campaigns"`
	Count        *int64          `json:"count"`
	StartElement *int            `json:"start_element"`
	NumElements  *int            `json:"num_elements"`
}

func (client *Client) doGET(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	options ...socialhub.CallOption,
) (responseWire, ResponseMeta, error) {
	callOptions, err := normalizedReadCallOptions(operation, options)
	if err != nil {
		return responseWire{}, ResponseMeta{}, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	for attempt := 0; attempt < 2; attempt++ {
		request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil)
		if err != nil {
			return responseWire{}, ResponseMeta{}, withOperation(err, operation)
		}
		failedToken := request.Header.Get("Authorization")
		request.Header.Set("Accept-Encoding", "identity")
		var raw json.RawMessage
		metadata, err := client.api.DoWithMetadata(request, &raw)
		if err != nil {
			err = withOperation(err, operation)
			if attempt == 0 && isNOAUTH(err) {
				client.sessions.Invalidate(failedToken)
				continue
			}
			return responseWire{}, ResponseMeta{}, err
		}
		if metadata.StatusCode != http.StatusOK || len(raw) == 0 || !json.Valid(raw) {
			return responseWire{}, ResponseMeta{}, platformContractError(operation, "Xandr returned an invalid successful HTTP response")
		}
		if !validJSONContentType(metadata.Header.Get("Content-Type")) {
			return responseWire{}, ResponseMeta{}, platformContractError(operation, "Xandr success response was not application/json")
		}
		var envelope apiEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Response == nil {
			return responseWire{}, ResponseMeta{}, platformContractError(operation, "Xandr returned an invalid response envelope")
		}
		wire := *envelope.Response
		meta := responseMetadata(metadata.Header, client.clock, client.sessions.requestIDs)
		if wire.ErrorID != "" {
			err := businessError(operation, metadata.StatusCode, metadata.Header, wire, client.clock.Now(), client.sessions.requestIDs)
			if attempt == 0 && wire.ErrorID == "NOAUTH" {
				client.sessions.Invalidate(failedToken)
				continue
			}
			return responseWire{}, meta, err
		}
		if wire.Error != "" || wire.Status != "OK" {
			return responseWire{}, meta, platformContractError(operation, "Xandr response omitted a documented OK or error status")
		}
		return wire, meta, nil
	}
	return responseWire{}, ResponseMeta{}, platformContractError(operation, "Xandr session retry was exhausted")
}

func normalizedReadCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, withOperation(err, operation)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "only per-call timeout is supported")
	}
	if resolved.Timeout < 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "call timeout must not be negative")
	}
	return resolved, nil
}

func responseMetadata(header http.Header, clock socialhub.Clock, requestIDs *requestIDFilter) ResponseMeta {
	return ResponseMeta{
		RateLimitCode:  boundedUnsignedHeader(header.Get("X-RateLimit-Code"), 32),
		RateLimitCount: boundedUnsignedHeader(header.Get("X-RateLimit-Count"), 32),
		RequestID:      responseRequestID(header, requestIDs), RetryAfter: retryDelay(header, clock.Now()),
	}
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func decodeRawList(operation, name string, raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, platformContractError(operation, "Xandr success response omitted "+name)
	}
	var values []json.RawMessage
	if err := json.Unmarshal(trimmed, &values); err != nil || values == nil {
		return nil, platformContractError(operation, "Xandr returned invalid "+name)
	}
	return values, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func isNOAUTH(err error) bool {
	var hub *socialhub.Error
	return errors.As(err, &hub) && strings.EqualFold(hub.PlatformCode, "NOAUTH")
}
