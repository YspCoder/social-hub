package peertube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthLocalClient is the public client credential discovered from one instance.
type OAuthLocalClient struct {
	ClientID     string
	ClientSecret string
}

// OAuthClient implements PeerTube's local-client discovery, password grant,
// and refresh-token grant.
type OAuthClient struct {
	InstanceURL string
	HTTPClient  *http.Client
	Clock       socialhub.Clock
}

// Discover retrieves the local OAuth client ID and secret published by an instance.
func (c *OAuthClient) Discover(ctx context.Context) (OAuthLocalClient, error) {
	if err := c.validate(); err != nil {
		return OAuthLocalClient{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizeInstanceURL(c.InstanceURL)+"/api/v1/oauth-clients/local", nil)
	if err != nil {
		return OAuthLocalClient{}, platformError("oauth_discover", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	body, err := c.do(request, "oauth_discover")
	if err != nil {
		return OAuthLocalClient{}, err
	}
	var payload oauthLocalClient
	if err := json.Unmarshal(body, &payload); err != nil {
		return OAuthLocalClient{}, platformError("oauth_discover", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOAuthCredential(payload.ClientID) || !validOAuthCredential(payload.ClientSecret) {
		return OAuthLocalClient{}, platformError("oauth_discover", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("invalid local OAuth client response"))
	}
	return OAuthLocalClient{ClientID: payload.ClientID, ClientSecret: payload.ClientSecret}, nil
}

// Password exchanges an activated user's credentials for access and refresh tokens.
// OTP is sent only through X-PeerTube-OTP and may be empty when 2FA is disabled.
func (c *OAuthClient) Password(ctx context.Context, client OAuthLocalClient, username, password, otp string) (socialhub.Token, error) {
	if strings.TrimSpace(username) == "" || password == "" {
		return socialhub.Token{}, invalidArgument("oauth_password", "username and password are required")
	}
	values := url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret}, "grant_type": {"password"},
		"username": {username}, "password": {password},
	}
	return c.token(ctx, "oauth_password", client, values, otp)
}

// Refresh exchanges a refresh token for a new access-token bundle.
func (c *OAuthClient) Refresh(ctx context.Context, client OAuthLocalClient, refreshToken string) (socialhub.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret}, "grant_type": {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	return c.token(ctx, "oauth_refresh", client, values, "")
}

func (c *OAuthClient) token(ctx context.Context, operation string, client OAuthLocalClient, values url.Values, otp string) (socialhub.Token, error) {
	if err := c.validate(); err != nil {
		return socialhub.Token{}, err
	}
	if !validOAuthCredential(client.ClientID) || !validOAuthCredential(client.ClientSecret) {
		return socialhub.Token{}, invalidArgument(operation, "valid local OAuth client credentials are required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeInstanceURL(c.InstanceURL)+"/api/v1/users/token", strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if otp != "" {
		request.Header.Set("X-PeerTube-OTP", otp)
	}
	body, err := c.do(request, operation)
	if err != nil {
		return socialhub.Token{}, err
	}
	var payload oauthTokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || len(payload.AccessToken) > 8192 || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((30*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("invalid token response"))
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: tokenType,
		ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: []string{"user"},
	}, nil
}

func (c *OAuthClient) validate() error {
	if c == nil || !validInstanceURL(c.InstanceURL) || c.HTTPClient == nil || c.Clock == nil {
		return invalidArgument("oauth", "instance URL, HTTP client, and clock are required")
	}
	return nil
}

func (c *OAuthClient) do(request *http.Request, operation string) ([]byte, error) {
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	return body, nil
}

func validOAuthCredential(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= 128
}
