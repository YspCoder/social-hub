package wechatstore

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type storeInfoWire struct {
	Nickname      string          `json:"nickname"`
	HeadImageURL  string          `json:"headimg_url"`
	SubjectType   ShopSubjectType `json:"subject_type"`
	Status        ShopStatus      `json:"status"`
	Username      string          `json:"username"`
	IsLocalLife   int             `json:"is_local_life"`
	OpenTimestamp int64           `json:"open_timestamp"`
}

type storeInfoResponse struct {
	Info storeInfoWire `json:"info"`
}

// GetInfo returns the documented, non-customer fields for the configured store.
func (client *Client) GetInfo(ctx context.Context, options ...socialhub.CallOption) (*StoreInfo, error) {
	const operation = "get_store_info"
	accessToken, err := client.accessToken(ctx, options...)
	if err != nil {
		return nil, err
	}
	query := url.Values{"access_token": {accessToken}}
	var response storeInfoResponse
	if err := client.doJSON(ctx, operation, http.MethodGet, "/channels/ec/basics/info/get", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := validateStoreInfo(response.Info); err != nil {
		return nil, err
	}
	return &StoreInfo{
		Nickname: response.Info.Nickname, HeadImageURL: response.Info.HeadImageURL,
		SubjectType: response.Info.SubjectType, Status: response.Info.Status,
		Username: response.Info.Username, IsLocalLife: response.Info.IsLocalLife == 1,
		OpenTimestamp: response.Info.OpenTimestamp,
	}, nil
}
