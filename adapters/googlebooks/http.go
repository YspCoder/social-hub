package googlebooks

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes int64 = 8 << 20

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	if ctx == nil {
		return ResponseMeta{}, invalidArgument(operation, "context must not be nil")
	}
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	apiKey, accessToken, err := client.credentials(operation)
	if err != nil {
		return ResponseMeta{}, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	requestQuery := make(url.Values, len(query)+1)
	for key, values := range query {
		requestQuery[key] = append([]string(nil), values...)
	}
	if apiKey != "" {
		requestQuery.Set("key", apiKey)
	}
	requestURL := baseURL + "/" + strings.TrimLeft(path, "/")
	if encoded := requestQuery.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return ResponseMeta{}, transportError(operation, err, apiKey, accessToken)
	}
	request.Header.Set("Accept", "application/json")
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ResponseMeta{}, transportError(operation, err, apiKey, accessToken)
	}
	defer response.Body.Close()
	meta := responseMeta(response.Header, client.clock, apiKey, accessToken)
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProviderObjectBytes+1))
	if err != nil {
		return meta, transportError(operation, err, apiKey, accessToken)
	}
	if int64(len(body)) > maxProviderObjectBytes {
		return meta, platformContractError(operation, "Google Books response exceeded the 8 MiB safety limit")
	}
	trimmed := bytes.TrimSpace(body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return meta, decodeAPIError(operation, response.StatusCode, response.Header, trimmed, client.clock, apiKey, accessToken)
	}
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return meta, platformContractError(operation, "Google Books returned an empty, invalid, or non-object JSON success response")
	}
	if !validJSONContentType(response.Header.Get("Content-Type")) {
		return meta, platformContractError(operation, "Google Books returned a non-JSON success response")
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(trimmed, &envelope) != nil {
		return meta, platformContractError(operation, "Google Books returned an invalid JSON success object")
	}
	if encoded, exists := envelope["error"]; exists && !bytes.Equal(bytes.TrimSpace(encoded), []byte("null")) {
		return meta, decodeAPIError(operation, response.StatusCode, response.Header, trimmed, client.clock, apiKey, accessToken)
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, nil
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
