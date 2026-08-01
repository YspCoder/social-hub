package snapchat

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

// OAuthClient implements Snapchat Marketing API's OAuth 2.0 authorization
// code and rotating refresh-token flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" || !contains(scopes, requiredScope) {
		return "", fmt.Errorf("snapchat: client ID, redirect URI, state, and snapchat-profile-api scope are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("snapchat: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if code == "" || redirectURI == "" {
		return socialhub.Token{}, fmt.Errorf("snapchat: code and redirect URI are required")
	}
	return c.token(ctx, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "code": {code},
		"redirect_uri": {redirectURI}, "grant_type": {"authorization_code"},
	}, "oauth_exchange", "")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if refreshToken == "" {
		return socialhub.Token{}, fmt.Errorf("snapchat: refresh token is required")
	}
	return c.token(ctx, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret},
		"refresh_token": {refreshToken}, "grant_type": {"refresh_token"},
	}, "oauth_refresh", refreshToken)
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation, retainedRefreshToken string) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil {
		return socialhub.Token{}, fmt.Errorf("snapchat: incomplete OAuth client")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		TokenType        string `json:"token_type"`
		ExpiresIn        int64  `json:"expires_in"`
		Scope            string `json:"scope"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.Error != "" {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "snapchat", Product: "snapchat-public-profile", Op: operation,
			PlatformCode: payload.Error, PlatformMessage: boundedMessage(payload.ErrorDescription, 512),
		}
	}
	if payload.AccessToken == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	tokenType := payload.TokenType
	if strings.EqualFold(tokenType, "bearer") || tokenType == "" {
		tokenType = "Bearer"
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: firstNonEmpty(payload.RefreshToken, retainedRefreshToken),
		TokenType: tokenType, ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope),
	}, nil
}
