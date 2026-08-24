package marketing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Magnetic Engine browser authorization and token
// exchange. The browser and token endpoints use different production origins.
type OAuthClient struct {
	AppID                int64
	Secret               string
	BaseURL              string
	AuthorizationBaseURL string
	HTTPClient           *http.Client
	Clock                socialhub.Clock
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
	if input.State != "" && !validOpaque(input.State, 4096) || !validOAuthType(input.OAuthType) || len(input.Scopes) > 100 {
		return "", invalidArgument("oauth_authorize", "state, OAuth type, or scopes are invalid")
	}
	for _, scope := range input.Scopes {
		if !validFieldName(scope) {
			return "", invalidArgument("oauth_authorize", "scopes must be lowercase API identifiers")
		}
	}
	query := url.Values{
		"app_id": {strconv.FormatInt(client.AppID, 10)}, "redirect_uri": {input.RedirectURI},
	}
	if len(input.Scopes) > 0 {
		encoded, _ := json.Marshal(input.Scopes)
		query.Set("scope", string(encoded))
	}
	if input.State != "" {
		query.Set("state", input.State)
	}
	if input.OAuthType != "" {
		query.Set("oauth_type", input.OAuthType)
	}
	return strings.TrimRight(client.AuthorizationBaseURL, "/") + "/tools/authorize?" + query.Encode(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, authorizationCode string) (OAuthToken, error) {
	if !validOpaque(authorizationCode, 4096) {
		return OAuthToken{}, invalidArgument("oauth_exchange", "auth_code is required")
	}
	return client.token(ctx, "oauth_exchange", "/oauth2/authorize/access_token", map[string]any{"auth_code": authorizationCode})
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (OAuthToken, error) {
	if !validOpaque(refreshToken, 8192) {
		return OAuthToken{}, invalidArgument("oauth_refresh", "refresh_token is required")
	}
	return client.token(ctx, "oauth_refresh", "/oauth2/authorize/refresh_token", map[string]any{"refresh_token": refreshToken})
}

func (client *OAuthClient) token(ctx context.Context, operation, path string, fields map[string]any) (OAuthToken, error) {
	if !validID(client.AppID) || !validOpaque(client.Secret, 4096) || !validEndpoint(client.BaseURL) ||
		client.HTTPClient == nil || client.Clock == nil {
		return OAuthToken{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	fields["app_id"], fields["secret"] = client.AppID, client.Secret
	encoded, err := json.Marshal(fields)
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(client.BaseURL, "/")+path, bytes.NewReader(encoded))
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
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return OAuthToken{}, platformContractError(operation, "OAuth response exceeded size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return OAuthToken{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	type tokenData struct {
		AccessToken           string `json:"access_token"`
		RefreshToken          string `json:"refresh_token"`
		AccessTokenExpiresIn  int64  `json:"access_token_expires_in"`
		RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
	}
	var envelope apiEnvelope[tokenData]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	data, err := requireEnvelope(operation, envelope, response.Header)
	if err != nil {
		return OAuthToken{}, err
	}
	if !validOpaque(data.AccessToken, 8192) || !validOpaque(data.RefreshToken, 8192) ||
		!validLifetime(data.AccessTokenExpiresIn) || !validLifetime(data.RefreshTokenExpiresIn) {
		return OAuthToken{}, platformContractError(operation, "OAuth response contains an invalid token or lifetime")
	}
	return OAuthToken{
		Token: socialhub.Token{
			AccessToken: data.AccessToken, RefreshToken: data.RefreshToken, TokenType: "KuaishouMagneticEngine",
			ExpiresAt: client.Clock.Now().Add(time.Duration(data.AccessTokenExpiresIn) * time.Second),
		},
		RefreshExpiresAt: client.Clock.Now().Add(time.Duration(data.RefreshTokenExpiresIn) * time.Second),
	}, nil
}

func sanitizeOAuthTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
