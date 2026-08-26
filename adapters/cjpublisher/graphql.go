package cjpublisher

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const maxGraphQLRequestBytes = 1 << 20

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse struct {
	Data       json.RawMessage `json:"data"`
	Errors     []GraphQLError  `json:"errors"`
	Extensions struct {
		RequestID string `json:"requestId"`
	} `json:"extensions"`
}

func (client *Client) doGraphQL(
	ctx context.Context,
	api *transport.Client,
	operation string,
	query string,
	variables map[string]any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, nil, err
	}
	encoded, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maxGraphQLRequestBytes {
		return ResponseMeta{}, nil, invalidArgument(operation, "GraphQL request exceeds 1 MiB")
	}
	request, err := api.NewRequest(ctx, http.MethodPost, "/query", nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	var raw json.RawMessage
	metadata, err := api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, nil, platformContractError(operation, "CJ returned an empty or invalid GraphQL response")
	}
	contentType := strings.TrimSpace(metadata.Header.Get("Content-Type"))
	if contentType != "" {
		mediaType, _, parseErr := mime.ParseMediaType(contentType)
		mediaType = strings.ToLower(mediaType)
		if parseErr != nil || mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
			return ResponseMeta{}, nil, platformContractError(operation, "CJ returned a non-JSON GraphQL response")
		}
	}
	var envelope graphQLResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	requestID := boundedMessage(firstNonEmpty(
		envelope.Extensions.RequestID,
		firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"),
		callOptions.RequestID,
	), 256)
	meta := ResponseMeta{RequestID: requestID}
	data := bytes.TrimSpace(envelope.Data)
	if len(data) > 0 && !bytes.Equal(data, []byte("null")) {
		if data[0] != '{' {
			return meta, append(json.RawMessage(nil), raw...), platformContractError(operation, "CJ GraphQL data is not an object")
		}
		if err := json.Unmarshal(data, output); err != nil {
			return meta, append(json.RawMessage(nil), raw...), platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	if len(envelope.Errors) > 0 {
		return meta, append(json.RawMessage(nil), raw...), graphQLOperationError(operation, envelope.Errors, raw, requestID)
	}
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return meta, append(json.RawMessage(nil), raw...), platformContractError(operation, "CJ GraphQL response omitted data without returning errors")
	}
	return meta, append(json.RawMessage(nil), raw...), nil
}
