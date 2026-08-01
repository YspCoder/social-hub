package kakao

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Kakao Login's OAuth 2.0 authorization-code and
// refresh-token flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// OAuthResult preserves Kakao fields that are not part of socialhub.Token.
type OAuthResult struct {
	Token            socialhub.Token `json:"token"`
	RefreshExpiresAt *time.Time      `json:"refresh_expires_at,omitempty"`
	IDToken          string          `json:"id_token,omitempty"`
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(state) == "" || !validHTTPURL(redirectURI) || !validEndpoint(c.AuthURL, true) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, and authorization URL are required")
	}
	for _, scope := range scopes {
		if !validBoundedString(scope, 256) || strings.ContainsAny(scope, ", ") {
			return "", invalidArgument("oauth_authorize", "OAuth scopes are invalid")
		}
	}
	parsed, _ := url.Parse(c.AuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	if len(scopes) > 0 {
		query.Set("scope", strings.Join(scopes, ","))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (OAuthResult, error) {
	if !validBoundedString(code, 4096) || !validHTTPURL(redirectURI) {
		return OAuthResult{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	values := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {c.ClientID},
		"redirect_uri": {redirectURI}, "code": {code},
	}
	c.addClientSecret(values)
	return c.token(ctx, values, "oauth_exchange", "")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (OAuthResult, error) {
	if !validBoundedString(refreshToken, 4096) {
		return OAuthResult{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"grant_type": {"refresh_token"}, "client_id": {c.ClientID},
		"refresh_token": {refreshToken},
	}
	c.addClientSecret(values)
	return c.token(ctx, values, "oauth_refresh", refreshToken)
}

func (c *OAuthClient) addClientSecret(values url.Values) {
	if c.ClientSecret != "" {
		values.Set("client_secret", c.ClientSecret)
	}
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation, retainedRefreshToken string) (OAuthResult, error) {
	if strings.TrimSpace(c.ClientID) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(c.TokenURL, true) {
		return OAuthResult{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return OAuthResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return OAuthResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return OAuthResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return OAuthResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return OAuthResult{}, decodeOAuthError(response.StatusCode, body, operation)
	}
	var payload struct {
		TokenType             string `json:"token_type"`
		AccessToken           string `json:"access_token"`
		ExpiresIn             int64  `json:"expires_in"`
		RefreshToken          string `json:"refresh_token"`
		RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
		Scope                 string `json:"scope"`
		IDToken               string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return OAuthResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	maxSeconds := int64((365 * 24 * time.Hour) / time.Second)
	if !validBoundedString(payload.AccessToken, 4096) || payload.ExpiresIn <= 0 || payload.ExpiresIn > maxSeconds ||
		(payload.RefreshToken != "" && !validBoundedString(payload.RefreshToken, 4096)) ||
		(payload.TokenType != "" && !validBoundedString(payload.TokenType, 64)) || len(payload.Scope) > 64<<10 || len(payload.IDToken) > 64<<10 {
		return OAuthResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = retainedRefreshToken
	}
	if refreshToken == "" {
		return OAuthResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	now := c.Clock.Now()
	result := OAuthResult{
		Token: socialhub.Token{
			AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: tokenType,
			ExpiresAt: now.Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: strings.Fields(payload.Scope),
		},
		IDToken: payload.IDToken,
	}
	if payload.RefreshTokenExpiresIn < 0 || payload.RefreshTokenExpiresIn > maxSeconds {
		return OAuthResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if payload.RefreshTokenExpiresIn > 0 {
		expiresAt := now.Add(time.Duration(payload.RefreshTokenExpiresIn) * time.Second)
		result.RefreshExpiresAt = &expiresAt
	}
	return result, nil
}

func decodeOAuthError(status int, body []byte, operation string) error {
	var response struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
		ErrorCode   string `json:"error_code"`
	}
	_ = json.Unmarshal(body, &response)
	code, class := socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	platformCode := response.ErrorCode
	if platformCode == "" {
		platformCode = response.Error
	}
	switch {
	case platformCode == "KOE237" || status == http.StatusTooManyRequests:
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case response.Error == "invalid_client" || response.Error == "invalid_grant" || status == http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case response.Error == "access_denied" || status == http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case response.Error == "temporarily_unavailable" || response.Error == "server_error" || status >= 500:
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "kakao", Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 128), PlatformMessage: boundedMessage(response.Description, 512),
	}
}
