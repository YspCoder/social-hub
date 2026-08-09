package baiduads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"social-hub/pkg/socialhub"
)

type requestHeader struct {
	UserName    string `json:"userName"`
	AccessToken string `json:"accessToken"`
}

type requestEnvelope struct {
	Header requestHeader `json:"header"`
	Body   any           `json:"body"`
}

func (client *Client) requestJSON(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) (http.Header, error) {
	wire := requestEnvelope{Header: requestHeader{UserName: client.userName, AccessToken: client.accessToken}, Body: input}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), options...)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("Accept", "application/json;charset=UTF-8")
	metadata, err := client.api.DoWithMetadata(request, output)
	return metadata.Header, withOperation(err, operation)
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

func oneResult[T any](operation string, values []T) (*T, error) {
	if len(values) != 1 {
		return nil, platformContractError(operation, "Baidu Ads returned an unexpected result count")
	}
	return &values[0], nil
}

func requireID(operation string, expected, actual int64) error {
	if actual <= 0 || expected > 0 && actual != expected {
		return platformContractError(operation, "Baidu Ads returned an invalid or mismatched resource ID")
	}
	return nil
}
