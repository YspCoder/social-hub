package wechatminiprogram

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type code2SessionResponse struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	SessionKey string `json:"session_key"`
}

// Code2Session exchanges one wx.login code for server-side login state. This
// flow is not OAuth and does not issue an application session to the caller.
func (client *Client) Code2Session(ctx context.Context, jsCode string, options ...socialhub.CallOption) (*Session, error) {
	const operation = "code2session"
	if !validSensitive(jsCode, maxCodeLength) {
		return nil, invalidArgument(operation, "wx.login code is required and invalid")
	}
	appID, secret, err := client.credentials(operation)
	if err != nil {
		return nil, err
	}
	query := url.Values{
		"appid":      {appID},
		"secret":     {secret},
		"js_code":    {jsCode},
		"grant_type": {"authorization_code"},
	}
	var response code2SessionResponse
	if err := client.doJSON(ctx, operation, http.MethodGet, "/sns/jscode2session", query, nil, &response, options...); err != nil {
		return nil, err
	}
	if !validSensitive(response.OpenID, maxOpenIDLength) ||
		!validSensitive(response.SessionKey, maxCredentialLength) ||
		!validOptionalSensitive(response.UnionID, maxOpenIDLength) {
		return nil, platformContractError(operation, "WeChat returned an incomplete login-state response")
	}
	return &Session{OpenID: response.OpenID, UnionID: response.UnionID, SessionKey: response.SessionKey}, nil
}
