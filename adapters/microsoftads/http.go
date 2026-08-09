package microsoftads

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) postJSON(ctx context.Context, operation string, api *transport.Client, path string, input, output any, options ...socialhub.CallOption) (http.Header, error) {
	if err := client.requireAccess(operation); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := api.DoWithMetadata(request, output)
	return metadata.Header, err
}

func (client *Client) putJSON(ctx context.Context, operation string, path string, input, output any, options ...socialhub.CallOption) (http.Header, error) {
	if err := client.requireAccess(operation); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.campaign.NewRequest(ctx, http.MethodPut, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	metadata, err := client.campaign.DoWithMetadata(request, output)
	return metadata.Header, err
}
