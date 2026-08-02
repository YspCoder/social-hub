package kitsu

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

func (c *Client) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validCredential(refreshToken) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is invalid")
	}
	if c.httpClient == nil || c.clock == nil || !validEndpoint(c.tokenURL) || !validUserAgent(c.userAgent) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "OAuth client is incomplete")
	}
	values := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	if c.clientID != "" {
		values.Set("client_id", c.clientID)
	}
	if c.clientSecret != "" {
		values.Set("client_secret", c.clientSecret)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeTransportError(err))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		decoded := decodeHTTPError(response.StatusCode, response.Header, body)
		if platformErr, ok := decoded.(*socialhub.Error); ok {
			platformErr.Op = "oauth_refresh"
		}
		return socialhub.Token{}, decoded
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		CreatedAt    int64  `json:"created_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validCredential(payload.AccessToken) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((366*24*time.Hour)/time.Second) ||
		(payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer")) {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	rotated := payload.RefreshToken
	if rotated == "" {
		rotated = refreshToken
	}
	if !validCredential(rotated) {
		return socialhub.Token{}, platformError("oauth_refresh", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	issuedAt := c.clock.Now()
	if payload.CreatedAt > 0 {
		issuedAt = time.Unix(payload.CreatedAt, 0)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: rotated, TokenType: "Bearer",
		ExpiresAt: issuedAt.Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}
