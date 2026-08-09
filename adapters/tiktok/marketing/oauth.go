package marketing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Marketing API advertiser authorization and exchange.
// The resulting long-term token has no refresh token or expiry timestamp.
type OAuthClient struct {
	AppID                string
	Secret               string
	BaseURL              string
	AuthorizationBaseURL string
	HTTPClient           *http.Client
}

func (client *OAuthClient) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if !validID(client.AppID) || !validEndpoint(client.AuthorizationBaseURL) {
		return "", invalidArgument("oauth_authorize", "OAuth client is incomplete")
	}
	redirect, err := url.Parse(input.RedirectURI)
	if err != nil || (redirect.Scheme != "https" && redirect.Scheme != "http") || redirect.Host == "" ||
		redirect.User != nil || redirect.Fragment != "" {
		return "", invalidArgument("oauth_authorize", "redirect_uri must be an absolute HTTP(S) URL without credentials or fragment")
	}
	if input.State != "" && !validOpaque(input.State, 4096) {
		return "", invalidArgument("oauth_authorize", "state is invalid")
	}
	query := url.Values{"app_id": {client.AppID}, "redirect_uri": {input.RedirectURI}}
	if input.State != "" {
		query.Set("state", input.State)
	}
	return strings.TrimRight(client.AuthorizationBaseURL, "/") + "/marketing_api/auth?" + query.Encode(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, authorizationCode string) (OAuthToken, error) {
	const operation = "oauth_exchange"
	if !validOpaque(authorizationCode, 4096) {
		return OAuthToken{}, invalidArgument(operation, "auth_code is required")
	}
	if !validID(client.AppID) || !validOpaque(client.Secret, 4096) ||
		!validEndpoint(client.BaseURL) || client.HTTPClient == nil {
		return OAuthToken{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	body := map[string]any{
		"app_id": client.AppID, "secret": client.Secret, "auth_code": authorizationCode,
		"return_advertiser_ids": true,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, strings.TrimRight(client.BaseURL, "/")+"/v1.3/oauth2/access_token/", bytes.NewReader(encoded),
	)
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeOAuthTransportError(err))
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(responseBody)) > maxOAuthResponseBytes {
		return OAuthToken{}, platformContractError(operation, "OAuth response exceeded size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return OAuthToken{}, decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	type tokenData struct {
		AccessToken           string   `json:"access_token"`
		AdvertiserIDs         []string `json:"advertiser_ids"`
		ScopeIDs              []int64  `json:"scope"`
		RefreshToken          string   `json:"refresh_token"`
		ExpiresIn             *int64   `json:"expires_in"`
		RefreshTokenExpiresIn *int64   `json:"refresh_token_expires_in"`
	}
	var envelope apiEnvelope[tokenData]
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	data, err := requireEnvelope(operation, envelope, response.Header)
	if err != nil {
		return OAuthToken{}, err
	}
	if !validOpaque(data.AccessToken, 8192) || !validateIDs(data.AdvertiserIDs, 100000) {
		return OAuthToken{}, platformContractError(operation, "OAuth response contains an invalid access token or advertiser ID")
	}
	for _, scopeID := range data.ScopeIDs {
		if scopeID <= 0 {
			return OAuthToken{}, platformContractError(operation, "OAuth response contains an invalid scope ID")
		}
	}
	if data.RefreshToken != "" || data.ExpiresIn != nil || data.RefreshTokenExpiresIn != nil {
		return OAuthToken{}, platformContractError(operation, "OAuth response used short-term user-token semantics instead of a Marketing long-term token")
	}
	return OAuthToken{
		Token:         socialhub.Token{AccessToken: data.AccessToken, TokenType: "Bearer"},
		AdvertiserIDs: append([]string(nil), data.AdvertiserIDs...), ScopeIDs: append([]int64(nil), data.ScopeIDs...),
	}, nil
}

func sanitizeOAuthTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
