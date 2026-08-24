package kakaomoment

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type creativeListResponse struct {
	Content []Creative `json:"content"`
}

func (client *Client) ListCreatives(ctx context.Context, adGroupID int64, configs []ConfigStatus, options ...socialhub.CallOption) ([]Creative, error) {
	const operation = "creative_list"
	if adGroupID <= 0 || !validConfigFilter(configs) {
		return nil, invalidArgument(operation, "Ad Group ID or Creative config filters are invalid")
	}
	query := url.Values{"adGroupId": {formatID(adGroupID)}}
	if len(configs) > 0 {
		query.Set("config", joinConfigs(configs))
	}
	var response creativeListResponse
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"creatives", query, nil, &response, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if response.Content == nil {
		return nil, platformContractError(operation, "Kakao Moment Creative response omitted content")
	}
	for index := range response.Content {
		if err := validateCreative(operation, &response.Content[index]); err != nil {
			return nil, err
		}
	}
	return response.Content, nil
}

func (client *Client) GetCreative(ctx context.Context, id int64, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_get"
	if id <= 0 {
		return nil, invalidArgument(operation, "Creative ID must be positive")
	}
	var creative Creative
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"creatives/"+formatID(id), nil, nil, &creative, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if err := validateCreative(operation, &creative); err != nil {
		return nil, err
	}
	if creative.ID != id {
		return nil, platformContractError(operation, "Kakao Moment returned a different Creative")
	}
	return &creative, nil
}

func (client *Client) SetCreativeConfig(ctx context.Context, id int64, config ConfigStatus, options ...socialhub.CallOption) error {
	const operation = "creative_set_config"
	if id <= 0 || !validConfig(config, false) {
		return invalidArgument(operation, "Creative ID must be positive and config must be ON or OFF")
	}
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodPut,
		"creatives/onOff", nil, configWire{ID: id, Config: config}, nil, true, options...,
	)
	return err
}

func (client *Client) DeleteCreative(ctx context.Context, id int64, options ...socialhub.CallOption) error {
	const operation = "creative_delete"
	if id <= 0 {
		return invalidArgument(operation, "Creative ID must be positive")
	}
	creative, err := client.GetCreative(ctx, id, options...)
	if err != nil {
		return withOperation(err, operation)
	}
	if creative.Config != ConfigOff {
		return conflict(operation, "Creative must be OFF before guarded deletion")
	}
	_, err = client.doJSON(
		ctx, operation, []string{ScopeManagement, ScopeDelete}, http.MethodDelete,
		"creatives/"+formatID(id), nil, nil, nil, true, options...,
	)
	return err
}

func validateCreative(operation string, creative *Creative) error {
	if creative == nil || creative.ID <= 0 || !validText(creative.Name, 1024) || !validConfig(creative.Config, true) {
		return platformContractError(operation, "Kakao Moment returned an invalid Creative")
	}
	return nil
}
