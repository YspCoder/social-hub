package linkedin

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

// OAuthClient implements LinkedIn's three-legged authorization-code flow.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" {
		return "", fmt.Errorf("linkedin: client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("linkedin: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("scope", strings.Join(scopes, " "))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if code == "" || redirectURI == "" {
		return socialhub.Token{}, fmt.Errorf("linkedin: code and redirect URI are required")
	}
	return c.token(ctx, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret},
	}, "oauth_exchange")
}

// Refresh exchanges a partner-enabled programmatic refresh token. Most apps
// must repeat the authorization flow because this feature is restricted.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if refreshToken == "" {
		return socialhub.Token{}, fmt.Errorf("linkedin: refresh token is required")
	}
	return c.token(ctx, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret},
	}, "oauth_refresh")
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation string) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil {
		return socialhub.Token{}, fmt.Errorf("linkedin: incomplete OAuth client")
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
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: tokenType,
		ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope),
	}, nil
}
