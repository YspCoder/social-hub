package zhihu

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

// OAuthClient implements Zhihu's documented OAuth 2.0 authorization-code flow.
type OAuthClient struct {
	AppID      string
	AppKey     string
	AuthURL    string
	TokenURL   string
	HTTPClient *http.Client
	Clock      socialhub.Clock
}

// AuthorizationURL returns the documented Zhihu authorization URL.
func (c *OAuthClient) AuthorizationURL(redirectURI string) (string, error) {
	if c.AppID == "" || redirectURI == "" {
		return "", invalidArgument("oauth_authorize", "app ID and redirect URI are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", invalidArgument("oauth_authorize", "authorization URL is invalid")
	}
	redirect, err := url.Parse(redirectURI)
	if err != nil || redirect.Scheme == "" || redirect.Host == "" {
		return "", invalidArgument("oauth_authorize", "redirect URI must be absolute")
	}
	query := parsed.Query()
	query.Set("redirect_uri", redirectURI)
	query.Set("app_id", c.AppID)
	query.Set("response_type", "code")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges a one-time authorization code for an OAuth access token.
func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if c.AppID == "" || c.AppKey == "" || c.TokenURL == "" || c.HTTPClient == nil || c.Clock == nil {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "OAuth client is incomplete")
	}
	if strings.TrimSpace(code) == "" || redirectURI == "" {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	form := url.Values{
		"app_id":       {c.AppID},
		"app_key":      {c.AppKey},
		"grant_type":   {"authorization_code"},
		"redirect_uri": {redirectURI},
		"code":         {code},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > maxResponseBytes {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.AccessToken == "" || payload.ExpiresIn <= 0 {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing token fields"))
	}
	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: tokenType, ExpiresAt: c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)}, nil
}
