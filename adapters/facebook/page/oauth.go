package page

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

// OAuthClient implements Meta's server-side authorization-code flow.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	APIURL       string
	HTTPClient   *http.Client
}

// PageAccess contains a Page access token derived from an authorized user token.
type PageAccess struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
}

// AuthorizationURL builds a Meta Login URL.
func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" {
		return "", fmt.Errorf("facebook page: client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("facebook page: invalid authorization URL")
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, ","))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// Exchange exchanges an authorization code for a user token.
func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if code == "" || redirectURI == "" {
		return socialhub.Token{}, fmt.Errorf("facebook page: code and redirect URI are required")
	}
	return c.token(ctx, url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"redirect_uri":  {redirectURI},
		"code":          {code},
	})
}

// ExchangeLongLived exchanges a short-lived user token for a long-lived token.
func (c *OAuthClient) ExchangeLongLived(ctx context.Context, shortLivedToken string) (socialhub.Token, error) {
	if shortLivedToken == "" {
		return socialhub.Token{}, fmt.Errorf("facebook page: short-lived token is required")
	}
	return c.token(ctx, url.Values{
		"grant_type":        {"fb_exchange_token"},
		"client_id":         {c.ClientID},
		"client_secret":     {c.ClientSecret},
		"fb_exchange_token": {shortLivedToken},
	})
}

// ListPages retrieves Pages managed by the authorized user and their Page
// access tokens.
func (c *OAuthClient) ListPages(ctx context.Context, userAccessToken string) ([]PageAccess, error) {
	if userAccessToken == "" || c.APIURL == "" || c.HTTPClient == nil {
		return nil, fmt.Errorf("facebook page: user token, API URL, and HTTP client are required")
	}
	endpoint := strings.TrimRight(c.APIURL, "/") + "/me/accounts?fields=id,name,access_token,tasks&limit=100"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+userAccessToken)
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, platformError("list_pages", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := readBounded(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, decodeError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		Data []PageAccess `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, platformError("list_pages", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return payload.Data, nil
}

func (c *OAuthClient) token(ctx context.Context, form url.Values) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.TokenURL == "" || c.HTTPClient == nil {
		return socialhub.Token{}, fmt.Errorf("facebook page: incomplete OAuth client")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return socialhub.Token{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := readBounded(response.Body)
	if err != nil {
		return socialhub.Token{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return socialhub.Token{}, platformError("oauth_token", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: payload.TokenType, ExpiresAt: time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)}, nil
}

func readBounded(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
	if err != nil {
		return nil, platformError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > 1<<20 {
		return nil, platformError("oauth_token", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return body, nil
}
