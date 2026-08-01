package kuaishou

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

// UserToken preserves Kuaishou's app-scoped open ID and refresh-token expiry.
type UserToken struct {
	Token            socialhub.Token
	OpenID           string
	RefreshExpiresAt time.Time
}

// OAuthClient implements Kuaishou's OAuth 2.0 authorization-code flow.
type OAuthClient struct {
	AppID        string
	AppSecret    string
	AuthURL      string
	TokenBaseURL string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// AuthorizationURL returns the web authorization URL.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.AppID == "" || redirectURI == "" || state == "" || len(scopes) == 0 {
		return "", fmt.Errorf("kuaishou: app ID, redirect URI, state, and scopes are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("kuaishou: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("app_id", c.AppID)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, ","))
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges a one-time authorization code for a user token.
func (c *OAuthClient) Exchange(ctx context.Context, code string) (UserToken, error) {
	if code == "" {
		return UserToken{}, fmt.Errorf("kuaishou: authorization code is required")
	}
	return c.tokenRequest(ctx, "/oauth2/access_token", url.Values{
		"app_id":     {c.AppID},
		"app_secret": {c.AppSecret},
		"code":       {code},
		"grant_type": {"authorization_code"},
	})
}

// Refresh rotates both tokens. Kuaishou invalidates the previous refresh token.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (UserToken, error) {
	if refreshToken == "" {
		return UserToken{}, fmt.Errorf("kuaishou: refresh token is required")
	}
	return c.tokenRequest(ctx, "/oauth2/refresh_token", url.Values{
		"app_id":        {c.AppID},
		"app_secret":    {c.AppSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	})
}

type tokenResponse struct {
	Result                int      `json:"result"`
	ErrorMessage          string   `json:"error_msg"`
	AccessToken           string   `json:"access_token"`
	RefreshToken          string   `json:"refresh_token"`
	OpenID                string   `json:"open_id"`
	ExpiresIn             int64    `json:"expires_in"`
	RefreshTokenExpiresIn int64    `json:"refresh_token_expires_in"`
	Scopes                []string `json:"scopes"`
}

func (c *OAuthClient) tokenRequest(ctx context.Context, path string, query url.Values) (UserToken, error) {
	if c.AppID == "" || c.AppSecret == "" || c.TokenBaseURL == "" || c.HTTPClient == nil || c.Clock == nil {
		return UserToken{}, fmt.Errorf("kuaishou: incomplete OAuth client")
	}
	endpoint, err := url.Parse(strings.TrimRight(c.TokenBaseURL, "/") + path)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return UserToken{}, fmt.Errorf("kuaishou: invalid token endpoint")
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return UserToken{}, wrapError("token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return UserToken{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return UserToken{}, wrapError("token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > 1<<20 {
		return UserToken{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return UserToken{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload tokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return UserToken{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := resultError(payload.Result, payload.ErrorMessage, "token", response.StatusCode, response.Header); err != nil {
		return UserToken{}, err
	}
	if payload.AccessToken == "" || payload.RefreshToken == "" {
		return UserToken{}, wrapError("token", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing token fields"))
	}
	now := c.Clock.Now()
	return UserToken{
		Token: socialhub.Token{
			AccessToken:  payload.AccessToken,
			RefreshToken: payload.RefreshToken,
			TokenType:    "Bearer",
			ExpiresAt:    now.Add(time.Duration(payload.ExpiresIn) * time.Second),
			Scopes:       append([]string(nil), payload.Scopes...),
		},
		OpenID:           payload.OpenID,
		RefreshExpiresAt: now.Add(time.Duration(payload.RefreshTokenExpiresIn) * time.Second),
	}, nil
}
