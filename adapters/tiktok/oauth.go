package tiktok

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

// OAuthClient implements TikTok Login Kit's current OAuth v2 endpoints.
type OAuthClient struct {
	ClientKey    string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

// TokenResult includes TikTok's app-scoped open_id and refresh expiry.
type TokenResult struct {
	Token            socialhub.Token
	OpenID           string
	RefreshExpiresAt time.Time
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	return c.authorizationURL(redirectURI, state, scopes, "")
}

// AuthorizationURLPKCE adds the S256 code challenge used by mobile and
// desktop authorization flows.
func (c *OAuthClient) AuthorizationURLPKCE(redirectURI, state string, scopes []string, codeChallenge string) (string, error) {
	if codeChallenge == "" {
		return "", fmt.Errorf("tiktok: code challenge is required")
	}
	return c.authorizationURL(redirectURI, state, scopes, codeChallenge)
}

func (c *OAuthClient) authorizationURL(redirectURI, state string, scopes []string, codeChallenge string) (string, error) {
	if c.ClientKey == "" || redirectURI == "" || state == "" {
		return "", fmt.Errorf("tiktok: client key, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("tiktok: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("client_key", c.ClientKey)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, ","))
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	if codeChallenge != "" {
		query.Set("code_challenge", codeChallenge)
		query.Set("code_challenge_method", "S256")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (TokenResult, error) {
	return c.ExchangeWithVerifier(ctx, code, redirectURI, "")
}

func (c *OAuthClient) ExchangeWithVerifier(ctx context.Context, code, redirectURI, codeVerifier string) (TokenResult, error) {
	if code == "" || redirectURI == "" {
		return TokenResult{}, fmt.Errorf("tiktok: code and redirect URI are required")
	}
	values := url.Values{
		"client_key": {c.ClientKey}, "client_secret": {c.ClientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}
	if codeVerifier != "" {
		values.Set("code_verifier", codeVerifier)
	}
	return c.token(ctx, values, "oauth_exchange")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (TokenResult, error) {
	if refreshToken == "" {
		return TokenResult{}, fmt.Errorf("tiktok: refresh token is required")
	}
	return c.token(ctx, url.Values{
		"client_key": {c.ClientKey}, "client_secret": {c.ClientSecret},
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}, "oauth_refresh")
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation string) (TokenResult, error) {
	if c.ClientKey == "" || c.ClientSecret == "" || c.HTTPClient == nil {
		return TokenResult{}, fmt.Errorf("tiktok: incomplete OAuth client")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return TokenResult{}, err
	}
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
		AccessToken      string `json:"access_token"`
		ExpiresIn        int64  `json:"expires_in"`
		OpenID           string `json:"open_id"`
		RefreshExpiresIn int64  `json:"refresh_expires_in"`
		RefreshToken     string `json:"refresh_token"`
		Scope            string `json:"scope"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		LogID            string `json:"log_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.Error != "" {
		return TokenResult{}, mapAPIError(response.StatusCode, response.Header, apiError{Code: payload.Error, Message: payload.ErrorDescription, LogID: payload.LogID})
	}
	if payload.AccessToken == "" || payload.OpenID == "" {
		return TokenResult{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := time.Now()
	var expiresAt, refreshExpiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = now.Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	if payload.RefreshExpiresIn > 0 {
		refreshExpiresAt = now.Add(time.Duration(payload.RefreshExpiresIn) * time.Second)
	}
	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return TokenResult{
		Token: socialhub.Token{
			AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: tokenType,
			ExpiresAt: expiresAt, Scopes: splitScopes(payload.Scope),
		},
		OpenID: payload.OpenID, RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func splitScopes(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
}
