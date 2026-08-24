package tencentads

import (
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

// OAuthClient implements Tencent Ads' browser authorization and token flows.
// Authorization and token endpoints use different production origins.
type OAuthClient struct {
	ClientID             int64
	ClientSecret         string
	AuthorizationBaseURL string
	TokenBaseURL         string
	HTTPClient           *http.Client
	Clock                socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(input AuthorizationRequest) (string, error) {
	if !validID(client.ClientID) || !validEndpoint(client.AuthorizationBaseURL) {
		return "", invalidArgument("oauth_authorize", "OAuth client is incomplete")
	}
	redirect, err := url.Parse(input.RedirectURI)
	if err != nil || (redirect.Scheme != "https" && redirect.Scheme != "http") || redirect.Host == "" ||
		redirect.User != nil || redirect.Port() != "" || redirect.Fragment != "" {
		return "", invalidArgument("oauth_authorize", "redirect_uri must be an absolute HTTP(S) URL without credentials, port, or fragment")
	}
	if input.State != "" && !validOpaque(input.State, 4096) || input.Scope != "" && !validOpaque(input.Scope, 4096) ||
		input.AccountType != "" && !validEnum(input.AccountType) || input.AccountDisplayNumber < 0 {
		return "", invalidArgument("oauth_authorize", "state, scope, account type, or account display number is invalid")
	}
	query := url.Values{
		"client_id": {strconv.FormatInt(client.ClientID, 10)}, "redirect_uri": {input.RedirectURI},
	}
	if input.State != "" {
		query.Set("state", input.State)
	}
	if input.Scope != "" {
		query.Set("scope", input.Scope)
	}
	if input.AccountType != "" {
		query.Set("account_type", input.AccountType)
	}
	if input.AccountDisplayNumber > 0 {
		query.Set("account_display_number", strconv.FormatInt(input.AccountDisplayNumber, 10))
	}
	if len(input.Fields) > 0 {
		for _, field := range input.Fields {
			if !validFieldName(field) {
				return "", invalidArgument("oauth_authorize", "fields must be lowercase API identifiers")
			}
		}
		if err := setJSONQuery(query, "fields", input.Fields, "oauth_authorize"); err != nil {
			return "", err
		}
	}
	return strings.TrimRight(client.AuthorizationBaseURL, "/") + "/oauth/authorize?" + query.Encode(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, authorizationCode, redirectURI string) (OAuthToken, error) {
	if !validOpaque(authorizationCode, 4096) {
		return OAuthToken{}, invalidArgument("oauth_exchange", "authorization_code is required")
	}
	if _, err := client.AuthorizationURL(AuthorizationRequest{RedirectURI: redirectURI}); err != nil {
		return OAuthToken{}, invalidArgument("oauth_exchange", "redirect_uri is invalid")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "authorization_code": {authorizationCode}, "redirect_uri": {redirectURI},
	})
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (OAuthToken, error) {
	if !validOpaque(refreshToken, 8192) {
		return OAuthToken{}, invalidArgument("oauth_refresh", "refresh_token is required")
	}
	return client.token(ctx, "oauth_refresh", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	})
}

func (client *OAuthClient) token(ctx context.Context, operation string, query url.Values) (OAuthToken, error) {
	if !validID(client.ClientID) || !validOpaque(client.ClientSecret, 4096) || !validEndpoint(client.TokenBaseURL) ||
		client.HTTPClient == nil || client.Clock == nil {
		return OAuthToken{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	query.Set("client_id", strconv.FormatInt(client.ClientID, 10))
	query.Set("client_secret", client.ClientSecret)
	requestURL := strings.TrimRight(client.TokenBaseURL, "/") + "/oauth/token?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return OAuthToken{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
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
		AuthorizerInfo        AuthorizerInfo `json:"authorizer_info"`
		AccessToken           string         `json:"access_token"`
		RefreshToken          string         `json:"refresh_token"`
		AccessTokenExpiresIn  int64          `json:"access_token_expires_in"`
		RefreshTokenExpiresIn int64          `json:"refresh_token_expires_in"`
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
		!validLifetime(data.AccessTokenExpiresIn) || !validLifetime(data.RefreshTokenExpiresIn) ||
		data.AuthorizerInfo.AccountID < 0 {
		return OAuthToken{}, platformContractError(operation, "OAuth response contains an invalid token, lifetime, or account ID")
	}
	return OAuthToken{
		Token: socialhub.Token{
			AccessToken: data.AccessToken, RefreshToken: data.RefreshToken, TokenType: "TencentAds",
			ExpiresAt: client.Clock.Now().Add(time.Duration(data.AccessTokenExpiresIn) * time.Second),
		},
		Authorizer:       data.AuthorizerInfo,
		RefreshExpiresAt: client.Clock.Now().Add(time.Duration(data.RefreshTokenExpiresIn) * time.Second),
	}, nil
}

func validLifetime(seconds int64) bool {
	return seconds > 0 && seconds <= int64((10*365*24*time.Hour)/time.Second)
}

func sanitizeOAuthTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}
