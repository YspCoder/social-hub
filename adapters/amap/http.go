package amap

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type providerResponse struct {
	Meta         ResponseMeta
	Places       []Place
	Raw          json.RawMessage
	CountPresent bool
}

type wireResponse struct {
	Status   json.RawMessage `json:"status"`
	Info     string          `json:"info"`
	InfoCode json.RawMessage `json:"infocode"`
	Count    json.RawMessage `json:"count"`
	Places   []Place         `json:"pois"`
}

func (client *Client) getPlaces(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	options ...socialhub.CallOption,
) (providerResponse, error) {
	requestOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return providerResponse{}, err
	}
	client.mu.RLock()
	if client.closed || client.api == nil {
		client.mu.RUnlock()
		return providerResponse{}, platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	api := client.api
	clock := client.clock
	secrets := append([]string(nil), client.secrets...)
	client.mu.RUnlock()
	request, err := api.NewRequest(ctx, http.MethodGet, path, query, nil, requestOptions...)
	if err != nil {
		return providerResponse{}, withOperation(err, operation)
	}
	var raw json.RawMessage
	metadata, err := api.DoWithMetadata(request, &raw)
	if err != nil {
		return providerResponse{Meta: ResponseMeta{HTTPStatus: metadata.StatusCode}}, withOperation(err, operation)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return providerResponse{}, platformContractError(operation, "Amap returned an empty, oversized, or invalid JSON response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return providerResponse{}, platformContractError(operation, "Amap returned a non-JSON response")
	}
	if providerErr, found := providerErrorFromBody(metadata.StatusCode, metadata.Header, trimmed, clock); found {
		return providerResponse{}, withOperation(providerErr, operation)
	}
	for _, secret := range secrets {
		if secret != "" && bytes.Contains(trimmed, []byte(secret)) {
			return providerResponse{}, platformContractError(operation, "Amap returned credential material in a success response")
		}
	}
	var decoded wireResponse
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return providerResponse{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	status, statusOK := scalarText(decoded.Status)
	infoCode, codeOK := scalarText(decoded.InfoCode)
	if !statusOK || !codeOK || status != "1" || infoCode != "10000" || decoded.Places == nil {
		return providerResponse{}, platformContractError(operation, "Amap returned an inconsistent success envelope")
	}
	count, countPresent, err := optionalNonnegativeInt(decoded.Count)
	if err != nil {
		return providerResponse{}, platformContractError(operation, "Amap returned an invalid count")
	}
	if !countPresent {
		count = len(decoded.Places)
	}
	return providerResponse{
		Meta: ResponseMeta{
			HTTPStatus: metadata.StatusCode, Status: status, Info: boundedMessage(decoded.Info, 1024),
			InfoCode: infoCode, Count: count,
		},
		Places: decoded.Places, Raw: append(json.RawMessage(nil), trimmed...), CountPresent: countPresent,
	}, nil
}

func scalarText(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", false
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		return strings.TrimSpace(text), true
	}
	var number json.Number
	if json.Unmarshal(trimmed, &number) == nil {
		return number.String(), true
	}
	return "", false
}

func optionalNonnegativeInt(raw json.RawMessage) (int, bool, error) {
	text, found := scalarText(raw)
	if !found {
		return 0, false, nil
	}
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		return 0, true, strconv.ErrSyntax
	}
	return value, true, nil
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
