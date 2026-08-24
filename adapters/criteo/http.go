package criteo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type apiEnvelope[T any] struct {
	Data     T         `json:"data"`
	Errors   []Problem `json:"errors"`
	Warnings []Problem `json:"warnings"`
}

func (client *Client) getJSON(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodGet, path, query, nil, output, options...), operation)
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPost, path, nil, input, output, options...), operation)
}

func (client *Client) patchJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return withOperation(client.api.JSON(ctx, http.MethodPatch, path, nil, input, output, options...), operation)
}

func (client *Client) postRawJSON(ctx context.Context, operation, path string, input any, options ...socialhub.CallOption) (json.RawMessage, string, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, "", invalidArgument(operation, "request cannot be encoded")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, "", withOperation(err, operation)
	}
	request.Header.Set("Content-Type", "application/json")
	var output json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &output)
	if err != nil {
		return nil, "", withOperation(err, operation)
	}
	if len(output) == 0 {
		return nil, "", platformContractError(operation, "Criteo returned an empty JSON report")
	}
	contentType := strings.TrimSpace(strings.Split(metadata.Header.Get("Content-Type"), ";")[0])
	if contentType != "" && contentType != "application/json" {
		return nil, "", platformContractError(operation, "Criteo returned a non-JSON report")
	}
	return output, contentType, nil
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

func checkProblems(operation string, problems []Problem) error {
	if len(problems) == 0 {
		return nil
	}
	return businessError(operation, problems)
}
