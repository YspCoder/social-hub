package thetradedesk

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

const maxRequestBytes = 1 << 20

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	method string,
	path string,
	query url.Values,
	input any,
	output any,
	mutation bool,
) (ResponseMeta, error) {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return ResponseMeta{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return ResponseMeta{}, invalidArgument(operation, "request JSON exceeds 1 MiB")
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	request.Header.Set("Accept-Encoding", "identity")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		err = withOperation(err, operation)
		if mutation && ambiguousMutationError(err) {
			return ResponseMeta{}, outcomeUnknownError(operation, err, "", client.requestIDs)
		}
		return ResponseMeta{}, err
	}
	resultMeta := ResponseMeta{RequestID: responseRequestID(metadata.Header, client.requestIDs)}
	if metadata.StatusCode != http.StatusOK {
		err := platformContractError(operation, "Platform API returned an unexpected successful HTTP status")
		if mutation {
			return resultMeta, outcomeUnknownError(operation, err, resultMeta.RequestID, client.requestIDs)
		}
		return resultMeta, err
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		err := platformContractError(operation, "Platform API success response was not application/json")
		if mutation {
			return resultMeta, outcomeUnknownError(operation, err, resultMeta.RequestID, client.requestIDs)
		}
		return resultMeta, err
	}
	if len(raw) == 0 {
		err := platformContractError(operation, "Platform API success response omitted JSON data")
		if mutation {
			return resultMeta, outcomeUnknownError(operation, err, resultMeta.RequestID, client.requestIDs)
		}
		return resultMeta, err
	}
	if output == nil {
		err := platformContractError(operation, "Platform API returned unexpected JSON data")
		if mutation {
			return resultMeta, outcomeUnknownError(operation, err, resultMeta.RequestID, client.requestIDs)
		}
		return resultMeta, err
	}
	if err := json.Unmarshal(raw, output); err != nil {
		contractErr := platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		if mutation {
			return resultMeta, outcomeUnknownError(operation, contractErr, resultMeta.RequestID, client.requestIDs)
		}
		return resultMeta, contractErr
	}
	return resultMeta, nil
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}
