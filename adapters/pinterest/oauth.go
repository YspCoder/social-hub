package pinterest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Pinterest OAuth 2.0 authorization-code, refresh-token,
// and client-credentials grants.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

// TokenResult includes Pinterest's continuous refresh-token expiry.
type TokenResult struct {
	Token            socialhub.Token
	ResponseType     string
	RefreshExpiresAt time.Time
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" || len(scopes) == 0 {
		return "", fmt.Errorf("pinterest: client ID, redirect URI, state, and scopes are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("pinterest: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, ","))
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (TokenResult, error) {
	if code == "" || redirectURI == "" {
		return TokenResult{}, fmt.Errorf("pinterest: code and redirect URI are required")
	}
	return c.token(ctx, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
	}, "oauth_exchange")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string, scopes []string) (TokenResult, error) {
	if refreshToken == "" {
		return TokenResult{}, fmt.Errorf("pinterest: refresh token is required")
	}
	values := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	if len(scopes) > 0 {
		values.Set("scope", strings.Join(scopes, ","))
	}
	return c.token(ctx, values, "oauth_refresh")
}

func (c *OAuthClient) ClientCredentials(ctx context.Context, scopes []string) (TokenResult, error) {
	if len(scopes) == 0 {
		return TokenResult{}, fmt.Errorf("pinterest: scopes are required")
	}
	return c.token(ctx, url.Values{"grant_type": {"client_credentials"}, "scope": {strings.Join(scopes, ",")}}, "oauth_client_credentials")
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation string) (TokenResult, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil {
		return TokenResult{}, fmt.Errorf("pinterest: incomplete OAuth client")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return TokenResult{}, err
	}
	request.SetBasicAuth(c.ClientID, c.ClientSecret)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return TokenResult{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken           string `json:"access_token"`
		RefreshToken          string `json:"refresh_token"`
		TokenType             string `json:"token_type"`
		ResponseType          string `json:"response_type"`
		ExpiresIn             int64  `json:"expires_in"`
		RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
		RefreshTokenExpiresAt int64  `json:"refresh_token_expires_at"`
		Scope                 string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	now := time.Now()
	var expiresAt, refreshExpiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = now.Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	if payload.RefreshTokenExpiresAt > 0 {
		refreshExpiresAt = time.Unix(payload.RefreshTokenExpiresAt, 0)
	} else if payload.RefreshTokenExpiresIn > 0 {
		refreshExpiresAt = now.Add(time.Duration(payload.RefreshTokenExpiresIn) * time.Second)
	}
	tokenType := payload.TokenType
	if strings.EqualFold(tokenType, "bearer") || tokenType == "" {
		tokenType = "Bearer"
	}
	return TokenResult{
		Token:        socialhub.Token{AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: tokenType, ExpiresAt: expiresAt, Scopes: splitScopes(payload.Scope)},
		ResponseType: payload.ResponseType, RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func splitScopes(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
}
