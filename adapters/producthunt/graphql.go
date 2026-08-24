package producthunt

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxGraphQLRequestBytes = 1 << 20

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors"`
}

func (client *Client) doGraphQL(
	ctx context.Context,
	operation string,
	query string,
	variables map[string]any,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, nil, err
	}
	encoded, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maxGraphQLRequestBytes {
		return ResponseMeta{}, nil, invalidArgument(operation, "GraphQL request exceeds 1 MiB")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/graphql", nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	meta := responseMeta(metadata.Header)
	if len(raw) == 0 || !json.Valid(raw) {
		return meta, nil, platformContractError(operation, "Product Hunt returned an empty or invalid GraphQL response")
	}
	if client.accessToken != "" && bytes.Contains(raw, []byte(client.accessToken)) {
		return meta, nil, platformContractError(operation, "Product Hunt reflected the configured access token in its response")
	}
	contentType, _, contentTypeErr := mime.ParseMediaType(metadata.Header.Get("Content-Type"))
	contentType = strings.ToLower(contentType)
	if contentTypeErr != nil || contentType != "application/json" && !strings.HasSuffix(contentType, "+json") {
		return meta, sanitizeProviderBody(raw, client.accessToken), platformContractError(operation, "Product Hunt returned a non-JSON GraphQL response")
	}
	var envelope graphQLResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return meta, sanitizeProviderBody(raw, client.accessToken), platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	data := bytes.TrimSpace(envelope.Data)
	if len(data) > 0 && !bytes.Equal(data, []byte("null")) {
		if data[0] != '{' {
			return meta, sanitizeProviderBody(raw, client.accessToken), platformContractError(operation, "Product Hunt GraphQL data is not an object")
		}
		if err := json.Unmarshal(data, output); err != nil {
			return meta, sanitizeProviderBody(raw, client.accessToken), platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
	}
	if len(envelope.Errors) > 0 {
		safeRaw := sanitizeProviderBody(raw, client.accessToken)
		return meta, safeRaw, graphQLOperationError(operation, metadata.StatusCode, envelope.Errors, raw, meta, client.accessToken)
	}
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return meta, sanitizeProviderBody(raw, client.accessToken), platformContractError(operation, "Product Hunt GraphQL response omitted data without returning errors")
	}
	return meta, append(json.RawMessage(nil), raw...), nil
}

func responseMeta(header http.Header) ResponseMeta {
	return ResponseMeta{
		RequestID:          boundedMessage(header.Get("X-Request-ID"), 256),
		RateLimitLimit:     boundedMessage(header.Get("X-Rate-Limit-Limit"), 64),
		RateLimitRemaining: boundedMessage(header.Get("X-Rate-Limit-Remaining"), 64),
		RateLimitReset:     boundedMessage(header.Get("X-Rate-Limit-Reset"), 64),
	}
}
