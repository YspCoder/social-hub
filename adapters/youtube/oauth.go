package youtube

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

// OAuthClient implements Google's web-server OAuth 2.0 flow.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" {
		return "", fmt.Errorf("youtube: client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("youtube: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	query.Set("access_type", "offline")
	query.Set("include_granted_scopes", "true")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if code == "" || redirectURI == "" {
		return socialhub.Token{}, fmt.Errorf("youtube: code and redirect URI are required")
	}
	return c.token(ctx, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}, "oauth_exchange")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if refreshToken == "" {
		return socialhub.Token{}, fmt.Errorf("youtube: refresh token is required")
	}
	return c.token(ctx, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret},
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}, "oauth_refresh")
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation string) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil {
		return socialhub.Token{}, fmt.Errorf("youtube: incomplete OAuth client")
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
	var payload struct {
		AccessToken      string `json:"access_token"`
		ExpiresIn        int64  `json:"expires_in"`
		RefreshToken     string `json:"refresh_token"`
		Scope            string `json:"scope"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || payload.Error != "" {
		code := socialhub.CodeUnauthenticated
		if payload.Error == "invalid_request" {
			code = socialhub.CodeInvalidArgument
		}
		return socialhub.Token{}, &socialhub.Error{Code: code, Class: socialhub.ClassUserAction, Platform: "youtube", Product: "youtube-data", Op: operation, HTTPStatus: response.StatusCode, PlatformCode: payload.Error, PlatformMessage: boundedMessage(payload.ErrorDescription, 512)}
	}
	if payload.AccessToken == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return socialhub.Token{AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: firstNonEmpty(payload.TokenType, "Bearer"), ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope)}, nil
}
