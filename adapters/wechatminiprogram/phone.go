package wechatminiprogram

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type phoneNumberResponse struct {
	PhoneInfo PhoneInfo `json:"phone_info"`
}

// Exchange consumes one getPhoneNumber code and returns verified personal
// data. It does not store, log, or retain a raw response.
func (client *Client) Exchange(ctx context.Context, input PhoneNumberRequest, options ...socialhub.CallOption) (*PhoneInfo, error) {
	const operation = "exchange_phone_number"
	if err := validatePhoneRequest(input); err != nil {
		return nil, err
	}
	appID, _, err := client.credentials(operation)
	if err != nil {
		return nil, err
	}
	accessToken, err := client.accessToken(ctx, options...)
	if err != nil {
		return nil, err
	}
	query := url.Values{"access_token": {accessToken}}
	var response phoneNumberResponse
	if err := client.doJSON(ctx, operation, http.MethodPost, "/wxa/business/getuserphonenumber", query, input, &response, options...); err != nil {
		return nil, err
	}
	if err := validatePhoneInfo(response.PhoneInfo, appID); err != nil {
		return nil, err
	}
	return &response.PhoneInfo, nil
}
