package imgur

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

type apiEnvelope struct {
	Data    json.RawMessage `json:"data"`
	Success bool            `json:"success"`
	Status  int             `json:"status"`
}

func (client *Client) request(ctx context.Context, api *transport.Client, method, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	var envelope apiEnvelope
	if err := api.JSON(ctx, method, path, query, nil, &envelope, options...); err != nil {
		return err
	}
	return decodeEnvelope(envelope, output)
}

func (client *Client) form(ctx context.Context, api *transport.Client, method, path string, values url.Values, output any, options ...socialhub.CallOption) error {
	request, err := api.NewRequest(ctx, method, path, nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var envelope apiEnvelope
	if err := api.Do(request, &envelope); err != nil {
		return err
	}
	return decodeEnvelope(envelope, output)
}

func (client *Client) basic(ctx context.Context, api *transport.Client, method, path string, values url.Values, options ...socialhub.CallOption) error {
	var success bool
	var err error
	if values == nil {
		err = client.request(ctx, api, method, path, nil, &success, options...)
	} else {
		err = client.form(ctx, api, method, path, values, &success, options...)
	}
	if err != nil {
		return err
	}
	if !success {
		return platformError(method+" "+path, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}

func decodeEnvelope(envelope apiEnvelope, output any) error {
	if !envelope.Success || envelope.Status < 200 || envelope.Status >= 300 {
		return imgurError(envelope.Status, nil, envelope.Data)
	}
	if output == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return platformError("decode_response", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if err := json.Unmarshal(envelope.Data, output); err != nil {
		return platformError("decode_response", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func path(parts ...string) string {
	return "/" + strings.Join(parts, "/")
}
