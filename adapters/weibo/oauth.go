package weibo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// OAuthClient implements Weibo OAuth2 Authorization Code exchange.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// AuthorizationURL returns the URL where a user grants access.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" {
		return "", fmt.Errorf("weibo: client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("weibo: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("state", state)
	if len(scopes) > 0 {
		query.Set("scope", strings.Join(scopes, ","))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges a one-time authorization code for an access token.
func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.TokenURL == "" || c.HTTPClient == nil || c.Clock == nil {
		return socialhub.Token{}, fmt.Errorf("weibo: incomplete OAuth client")
	}
	if code == "" || redirectURI == "" {
		return socialhub.Token{}, fmt.Errorf("weibo: code and redirect URI are required")
	}
	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > 1<<20 {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		APIError
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"`
		UID         string          `json:"uid"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if err := payload.APIError.Err("oauth_exchange", response.StatusCode, response.Header); err != nil {
		return socialhub.Token{}, err
	}
	if payload.AccessToken == "" {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("missing access token"))
	}
	expiresIn, err := parseJSONInt64(payload.ExpiresIn)
	if err != nil {
		return socialhub.Token{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	token := socialhub.Token{AccessToken: payload.AccessToken, TokenType: "Bearer"}
	if expiresIn > 0 {
		token.ExpiresAt = c.Clock.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	return token, nil
}

func parseJSONInt64(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	value := strings.Trim(string(raw), `"`)
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expires_in")
	}
	return parsed, nil
}
