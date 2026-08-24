package kakaomoment

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	requiredScopes []string,
	method string,
	path string,
	query url.Values,
	input any,
	output any,
	mutation bool,
	options ...socialhub.CallOption,
) (string, error) {
	if err := client.requireScopes(operation, requiredScopes...); err != nil {
		return "", err
	}
	safeOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return "", err
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return "", platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return "", invalidArgument(operation, "request JSON exceeds 1 MiB")
		}
		body = bytes.NewReader(encoded)
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, safeOptions...)
	if err != nil {
		return "", withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	if input != nil {
		request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		err = withOperation(err, operation)
		if mutation && ambiguousMutationError(err) {
			return "", outcomeUnknownError(operation, err, "")
		}
		return "", err
	}
	requestID := boundedOpaque(firstNonEmpty(
		metadata.Header.Get("X-Kakao-Request-Id"), metadata.Header.Get("X-Request-Id"), metadata.Header.Get("X-Correlation-Id"),
	), 256)
	if metadata.StatusCode != http.StatusOK {
		contractErr := platformContractError(operation, "Kakao Moment returned an unexpected successful HTTP status")
		if mutation {
			return requestID, outcomeUnknownError(operation, contractErr, requestID)
		}
		return requestID, contractErr
	}
	if len(raw) == 0 {
		if output == nil {
			return requestID, nil
		}
		contractErr := platformContractError(operation, "Kakao Moment success response omitted JSON data")
		if mutation {
			return requestID, outcomeUnknownError(operation, contractErr, requestID)
		}
		return requestID, contractErr
	}
	if !json.Valid(raw) {
		contractErr := platformContractError(operation, "Kakao Moment returned invalid JSON")
		if mutation {
			return requestID, outcomeUnknownError(operation, contractErr, requestID)
		}
		return requestID, contractErr
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil && object != nil {
		if codeRaw, exists := object["code"]; exists {
			var code int
			if json.Unmarshal(codeRaw, &code) != nil {
				contractErr := platformContractError(operation, "Kakao Moment returned an invalid business code")
				if mutation {
					return requestID, outcomeUnknownError(operation, contractErr, requestID)
				}
				return requestID, contractErr
			}
			if code != 0 && code != http.StatusOK {
				var envelope errorEnvelope
				if json.Unmarshal(raw, &envelope) != nil {
					return requestID, platformContractError(operation, "Kakao Moment returned an invalid error envelope")
				}
				return requestID, apiErrorValue(operation, metadata.StatusCode, metadata.Header, envelope, client.clock.Now())
			}
		}
	}
	if output == nil {
		contractErr := platformContractError(operation, "Kakao Moment mutation returned unexpected JSON data")
		if mutation {
			return requestID, outcomeUnknownError(operation, contractErr, requestID)
		}
		return requestID, contractErr
	}
	if err := json.Unmarshal(raw, output); err != nil {
		contractErr := platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		if mutation {
			return requestID, outcomeUnknownError(operation, contractErr, requestID)
		}
		return requestID, contractErr
	}
	return requestID, nil
}
