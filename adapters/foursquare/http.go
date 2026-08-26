package foursquare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (transport.ResponseMetadata, json.RawMessage, error) {
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, nil, withOperation(err, operation)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return metadata, nil, withOperation(err, operation)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return metadata, nil, platformContractError(operation, "Foursquare returned an empty or invalid successful response")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return metadata, nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return metadata, append(json.RawMessage(nil), raw...), nil
}
