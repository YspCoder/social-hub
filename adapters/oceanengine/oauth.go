package oceanengine

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Ocean Engine's token endpoints. Customer
// authorization itself starts from the URL configured in the developer console.
type OAuthClient struct {
	AppID      int64
	Secret     string
	BaseURL    string
	HTTPClient *http.Client
	Clock      socialhub.Clock
}

func (client *OAuthClient) Exchange(ctx context.Context, authCode string) (OAuthToken, error) {
	if !validOpaque(authCode, 4096) {
		return OAuthToken{}, invalidArgument("oauth_exchange", "auth_code is required")
	}
	return client.userToken(ctx, "oauth_exchange", "/open_api/oauth2/access_token/", map[string]any{
		"app_id": client.AppID, "secret": client.Secret, "auth_code": authCode,
	})
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (OAuthToken, error) {
	if !validOpaque(refreshToken, 8192) {
		return OAuthToken{}, invalidArgument("oauth_refresh", "refresh_token is required")
	}
	return client.userToken(ctx, "oauth_refresh", "/open_api/oauth2/refresh_token/", map[string]any{
		"app_id": client.AppID, "secret": client.Secret, "refresh_token": refreshToken,
	})
}

func (client *OAuthClient) Renew(ctx context.Context, refreshToken string) (OAuthToken, error) {
	if !validOpaque(refreshToken, 8192) {
		return OAuthToken{}, invalidArgument("oauth_renew", "refresh_token is required")
	}
	return client.userToken(ctx, "oauth_renew", "/open_api/oauth2/renew_token/", map[string]any{
		"app_id": client.AppID, "secret": client.Secret, "refresh_token": refreshToken,
	})
}

func (client *OAuthClient) AppToken(ctx context.Context) (socialhub.Token, error) {
	type responseData struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	var response apiEnvelope[responseData]
	if err := client.tokenRequest(ctx, "oauth_app_token", "/open_api/oauth2/app_access_token/", map[string]any{
		"app_id": client.AppID, "secret": client.Secret,
	}, &response); err != nil {
		return socialhub.Token{}, err
	}
	data, err := requireEnvelope("oauth_app_token", response)
	if err != nil {
		return socialhub.Token{}, err
	}
	if !validOpaque(data.AccessToken, 8192) || !validLifetime(data.ExpiresIn) {
		return socialhub.Token{}, platformContractError("oauth_app_token", "OAuth response contains an invalid token or lifetime")
	}
	return socialhub.Token{
		AccessToken: data.AccessToken, TokenType: "OceanEngineApp",
		ExpiresAt: expiry(client.Clock, data.ExpiresIn),
	}, nil
}

func (client *OAuthClient) userToken(ctx context.Context, operation, path string, input map[string]any) (OAuthToken, error) {
	type responseData struct {
		AccessToken           string  `json:"access_token"`
		RefreshToken          string  `json:"refresh_token"`
		AdvertiserIDs         []int64 `json:"advertiser_ids"`
		ExpiresIn             int64   `json:"expires_in"`
		RefreshTokenExpiresIn int64   `json:"refresh_token_expires_in"`
	}
	var response apiEnvelope[responseData]
	if err := client.tokenRequest(ctx, operation, path, input, &response); err != nil {
		return OAuthToken{}, err
	}
	data, err := requireEnvelope(operation, response)
	if err != nil {
		return OAuthToken{}, err
	}
	if !validOpaque(data.AccessToken, 8192) || !validOpaque(data.RefreshToken, 8192) ||
		!validLifetime(data.ExpiresIn) || !validLifetime(data.RefreshTokenExpiresIn) || !validateIDsUnbounded(data.AdvertiserIDs) {
		return OAuthToken{}, platformContractError(operation, "OAuth response contains an invalid token, lifetime, or advertiser ID")
	}
	return OAuthToken{
		Token: socialhub.Token{
			AccessToken: data.AccessToken, RefreshToken: data.RefreshToken,
			TokenType: "OceanEngineUser", ExpiresAt: expiry(client.Clock, data.ExpiresIn),
		},
		AdvertiserIDs:    append([]int64(nil), data.AdvertiserIDs...),
		RefreshExpiresAt: expiry(client.Clock, data.RefreshTokenExpiresIn),
	}, nil
}

func (client *OAuthClient) tokenRequest(ctx context.Context, operation, path string, input, output any) error {
	if !validID(client.AppID) || !validOpaque(client.Secret, 4096) || !validEndpoint(client.BaseURL) || client.HTTPClient == nil || client.Clock == nil {
		return invalidArgument(operation, "OAuth client is incomplete")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(responseBody)) > maxOAuthResponseBytes {
		return platformContractError(operation, "OAuth response exceeded size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	if err := json.Unmarshal(responseBody, output); err != nil {
		return platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func validLifetime(seconds int64) bool {
	return seconds > 0 && seconds <= int64((10*365*24*time.Hour)/time.Second)
}

func expiry(clock socialhub.Clock, seconds int64) time.Time {
	return clock.Now().Add(time.Duration(seconds) * time.Second)
}

func validateIDsUnbounded(values []int64) bool {
	for _, value := range values {
		if !validID(value) {
			return false
		}
	}
	return true
}
