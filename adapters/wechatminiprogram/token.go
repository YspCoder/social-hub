package wechatminiprogram

import (
	"context"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	stableTokenRefreshSkew = 5 * time.Minute
	forceRefreshInterval   = 30 * time.Second
)

type stableTokenRequest struct {
	GrantType    string `json:"grant_type"`
	AppID        string `json:"appid"`
	Secret       string `json:"secret"`
	ForceRefresh bool   `json:"force_refresh"`
}

type stableTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// GetStableAccessToken returns the cached stable token or retrieves one in
// ordinary mode. Value is sensitive and must never be logged.
func (client *Client) GetStableAccessToken(ctx context.Context, options ...socialhub.CallOption) (*StableAccessToken, error) {
	return client.stableAccessToken(ctx, false, options...)
}

// ForceRefreshStableAccessToken invalidates the previous provider token. It
// should be used only for explicit credential recovery.
func (client *Client) ForceRefreshStableAccessToken(ctx context.Context, options ...socialhub.CallOption) (*StableAccessToken, error) {
	return client.stableAccessToken(ctx, true, options...)
}

func (client *Client) accessToken(ctx context.Context, options ...socialhub.CallOption) (string, error) {
	token, err := client.stableAccessToken(ctx, false, options...)
	if err != nil {
		return "", err
	}
	return token.Value, nil
}

func (client *Client) stableAccessToken(ctx context.Context, forceRefresh bool, options ...socialhub.CallOption) (*StableAccessToken, error) {
	operation := "get_stable_access_token"
	if forceRefresh {
		operation = "force_refresh_stable_access_token"
	}
	if _, err := prepareCallOptions(operation, options); err != nil {
		return nil, err
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed || client.appID == "" || client.appSecret == "" {
		return nil, platformError(operation, socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	now := client.clock.Now()
	if !forceRefresh && client.token.Value != "" && now.Add(stableTokenRefreshSkew).Before(client.token.ExpiresAt) {
		return tokenAt(client.token, now), nil
	}
	if forceRefresh && !client.lastForceRefresh.IsZero() {
		elapsed := now.Sub(client.lastForceRefresh)
		if elapsed < forceRefreshInterval {
			return nil, localRateLimitError(operation, forceRefreshInterval-elapsed)
		}
	}
	if forceRefresh {
		client.lastForceRefresh = now
	}
	input := stableTokenRequest{
		GrantType: "client_credential", AppID: client.appID,
		Secret: client.appSecret, ForceRefresh: forceRefresh,
	}
	var response stableTokenResponse
	if err := client.doJSON(ctx, operation, http.MethodPost, "/cgi-bin/stable_token", nil, input, &response, options...); err != nil {
		return nil, err
	}
	if !validSensitive(response.AccessToken, maxCredentialLength) || response.ExpiresIn <= 0 || response.ExpiresIn > 7_200 {
		return nil, platformContractError(operation, "WeChat returned an invalid stable access token response")
	}
	client.token = StableAccessToken{
		Value:     response.AccessToken,
		ExpiresAt: now.Add(time.Duration(response.ExpiresIn) * time.Second),
	}
	return tokenAt(client.token, now), nil
}

func tokenAt(token StableAccessToken, now time.Time) *StableAccessToken {
	copy := token
	copy.ExpiresIn = token.ExpiresAt.Sub(now)
	if copy.ExpiresIn < 0 {
		copy.ExpiresIn = 0
	}
	return &copy
}
