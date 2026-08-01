package twitch

import (
	"bytes"
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

// OAuthClient implements Twitch user and app OAuth2 flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	ValidateURL  string
	RevokeURL    string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// TokenValidation describes the current authorization bound to a token.
type TokenValidation struct {
	ClientID  string   `json:"client_id"`
	Login     string   `json:"login"`
	Scopes    []string `json:"scopes"`
	UserID    string   `json:"user_id"`
	ExpiresIn int64    `json:"expires_in"`
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string, forceVerify bool) (string, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(redirectURI) == "" || strings.TrimSpace(state) == "" {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", invalidArgument("oauth_authorize", "authorization URL is invalid")
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("state", state)
	query.Set("scope", strings.Join(scopes, " "))
	if forceVerify {
		query.Set("force_verify", "true")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(redirectURI) == "" {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code and redirect URI are required")
	}
	body, err := c.do(ctx, c.TokenURL, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}, "oauth_exchange")
	if err != nil {
		return socialhub.Token{}, err
	}
	return c.decodeToken(body, "oauth_exchange")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	body, err := c.do(ctx, c.TokenURL, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "grant_type": {"refresh_token"},
		"refresh_token": {refreshToken},
	}, "oauth_refresh")
	if err != nil {
		return socialhub.Token{}, err
	}
	return c.decodeToken(body, "oauth_refresh")
}

func (c *OAuthClient) ClientCredentials(ctx context.Context, scopes []string) (socialhub.Token, error) {
	body, err := c.do(ctx, c.TokenURL, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "grant_type": {"client_credentials"},
		"scope": {strings.Join(scopes, " ")},
	}, "oauth_client_credentials")
	if err != nil {
		return socialhub.Token{}, err
	}
	return c.decodeToken(body, "oauth_client_credentials")
}

// Validate checks token status. Twitch requires active third-party sessions to
// call this endpoint when they start and at least hourly thereafter.
func (c *OAuthClient) Validate(ctx context.Context, accessToken string) (*TokenValidation, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, invalidArgument("oauth_validate", "access token is required")
	}
	if c.HTTPClient == nil || !validEndpoint(c.ValidateURL) {
		return nil, invalidArgument("oauth_validate", "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ValidateURL, nil)
	if err != nil {
		return nil, platformError("oauth_validate", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Authorization", "OAuth "+accessToken)
	body, err := c.execute(request, "oauth_validate")
	if err != nil {
		return nil, err
	}
	var result TokenValidation
	if err := json.Unmarshal(body, &result); err != nil || result.ClientID == "" || result.ExpiresIn < 0 {
		return nil, platformError("oauth_validate", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &result, nil
}

func (c *OAuthClient) Revoke(ctx context.Context, accessToken string) error {
	if strings.TrimSpace(accessToken) == "" {
		return invalidArgument("oauth_revoke", "access token is required")
	}
	if c.HTTPClient == nil || !validEndpoint(c.RevokeURL) || strings.TrimSpace(c.ClientID) == "" {
		return invalidArgument("oauth_revoke", "OAuth client is incomplete")
	}
	form := url.Values{"client_id": {c.ClientID}, "token": {accessToken}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return platformError("oauth_revoke", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = c.execute(request, "oauth_revoke")
	return err
}

func (c *OAuthClient) do(ctx context.Context, endpoint string, values url.Values, operation string) ([]byte, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(endpoint) {
		return nil, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.execute(request, operation)
}

func (c *OAuthClient) execute(request *http.Request, operation string) ([]byte, error) {
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
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	return body, nil
}

func (c *OAuthClient) decodeToken(body []byte, operation string) (socialhub.Token, error) {
	var response struct {
		AccessToken  string   `json:"access_token"`
		RefreshToken string   `json:"refresh_token"`
		ExpiresIn    int64    `json:"expires_in"`
		Scopes       []string `json:"scope"`
		TokenType    string   `json:"token_type"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil || response.AccessToken == "" || response.ExpiresIn < 0 {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	tokenType := response.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	var expiresAt time.Time
	if response.ExpiresIn > 0 {
		if response.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
			return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		expiresAt = c.Clock.Now().Add(time.Duration(response.ExpiresIn) * time.Second)
	}
	return socialhub.Token{
		AccessToken: response.AccessToken, RefreshToken: response.RefreshToken, TokenType: tokenType,
		ExpiresAt: expiresAt, Scopes: append([]string(nil), response.Scopes...),
	}, nil
}
