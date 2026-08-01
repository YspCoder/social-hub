package giphy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type singleEnvelope[T any] struct {
	Data T    `json:"data"`
	Meta Meta `json:"meta"`
}

type listEnvelope[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
	Meta       Meta       `json:"meta"`
}

func (envelope *singleEnvelope[T]) UnmarshalJSON(data []byte) error {
	var raw struct {
		Data json.RawMessage `json:"data"`
		Meta Meta            `json:"meta"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	envelope.Meta = raw.Meta
	if raw.Meta.Status < 200 || raw.Meta.Status >= 300 || len(raw.Data) == 0 || string(raw.Data) == "null" {
		return nil
	}
	return json.Unmarshal(raw.Data, &envelope.Data)
}

func (envelope *listEnvelope[T]) UnmarshalJSON(data []byte) error {
	var raw struct {
		Data       json.RawMessage `json:"data"`
		Pagination Pagination      `json:"pagination"`
		Meta       Meta            `json:"meta"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	envelope.Meta, envelope.Pagination = raw.Meta, raw.Pagination
	if raw.Meta.Status < 200 || raw.Meta.Status >= 300 || len(raw.Data) == 0 || string(raw.Data) == "null" {
		return nil
	}
	return json.Unmarshal(raw.Data, &envelope.Data)
}

func (client *Client) get(ctx context.Context, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	if err := client.api.JSON(ctx, http.MethodGet, path, query, nil, output, options...); err != nil {
		return err
	}
	return nil
}

func checkMeta(operation string, meta Meta) error {
	if meta.Status < 200 || meta.Status >= 300 {
		return giphyError(operation, meta.Status, meta, nil)
	}
	if meta.Status == 0 {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return nil
}
