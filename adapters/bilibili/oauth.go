package bilibili

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

// OAuthClient implements Bilibili's OAuth 2.0 authorization-code flow.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenBaseURL string
	HTTPClient   *http.Client
}

// AuthorizationURL returns the current PC web authorization URL.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" {
		return "", fmt.Errorf("bilibili: client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("bilibili: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("gourl", redirectURI)
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges a one-time authorization code for a user token.
func (c *OAuthClient) Exchange(ctx context.Context, code string) (socialhub.Token, error) {
	if code == "" {
		return socialhub.Token{}, fmt.Errorf("bilibili: authorization code is required")
	}
	return c.tokenRequest(ctx, "/x/account-oauth2/v1/token", url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
	})
}

// Refresh rotates the single-use refresh token.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if refreshToken == "" {
		return socialhub.Token{}, fmt.Errorf("bilibili: refresh token is required")
	}
	return c.tokenRequest(ctx, "/x/account-oauth2/v1/refresh_token", url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

type tokenData struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int64    `json:"expires_in"`
	Scopes       []string `json:"scopes"`
}

func (c *OAuthClient) tokenRequest(ctx context.Context, path string, query url.Values) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.TokenBaseURL == "" || c.HTTPClient == nil {
		return socialhub.Token{}, fmt.Errorf("bilibili: incomplete OAuth client")
	}
	endpoint, err := url.Parse(strings.TrimRight(c.TokenBaseURL, "/") + path)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return socialhub.Token{}, fmt.Errorf("bilibili: invalid token endpoint")
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return socialhub.Token{}, wrapError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return socialhub.Token{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > 1<<20 {
		return socialhub.Token{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var envelope responseEnvelope[tokenData]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return socialhub.Token{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := envelope.Err("token", response.StatusCode, response.Header); err != nil {
		return socialhub.Token{}, err
	}
	if envelope.Data.AccessToken == "" || envelope.Data.RefreshToken == "" || envelope.Data.ExpiresIn <= 0 {
		return socialhub.Token{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing token fields"))
	}
	return socialhub.Token{
		AccessToken: envelope.Data.AccessToken, RefreshToken: envelope.Data.RefreshToken, TokenType: "BilibiliUser",
		ExpiresAt: time.Unix(envelope.Data.ExpiresIn, 0), Scopes: append([]string(nil), envelope.Data.Scopes...),
	}, nil
}
