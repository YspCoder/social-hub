package tumblr

import (
	"context"
	"encoding/json"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

type tumblrEnvelope struct {
	Meta     tumblrMeta       `json:"meta"`
	Response json.RawMessage  `json:"response"`
	Errors   []tumblrAPIError `json:"errors"`
}

func (c *Client) request(ctx context.Context, client *transport.Client, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	var envelope tumblrEnvelope
	if err := client.JSON(ctx, method, path, query, input, &envelope, options...); err != nil {
		return err
	}
	return decodeEnvelope(envelope, output)
}

func decodeEnvelope(envelope tumblrEnvelope, output any) error {
	if envelope.Meta.Status < 200 || envelope.Meta.Status >= 300 {
		return tumblrError(envelope.Meta.Status, nil, envelope.Meta, envelope.Errors)
	}
	if output == nil {
		return nil
	}
	if len(envelope.Response) == 0 || string(envelope.Response) == "null" {
		return platformError("decode_response", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if err := json.Unmarshal(envelope.Response, output); err != nil {
		return platformError("decode_response", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}
