package threads

import (
	"bytes"
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

// OAuthClient implements Threads authorization-code and long-lived-token flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	LongTokenURL string
	RefreshURL   string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

// TokenResult contains the app-scoped Threads user ID returned by exchange.
type TokenResult struct {
	Token  socialhub.Token
	UserID string
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" {
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
	query.Set("scope", strings.Join(scopes, ","))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (TokenResult, error) {
	if code == "" || redirectURI == "" {
		return TokenResult{}, invalidArgument("oauth_exchange", "code and redirect URI are required")
	}
	body, err := c.do(ctx, http.MethodPost, c.TokenURL, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "grant_type": {"authorization_code"},
		"redirect_uri": {redirectURI}, "code": {code},
	}, true)
	if err != nil {
		return TokenResult{}, err
	}
	var response struct {
		AccessToken string   `json:"access_token"`
		UserID      stringID `json:"user_id"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.AccessToken == "" || response.UserID == "" {
		return TokenResult{}, platformError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return TokenResult{Token: socialhub.Token{AccessToken: response.AccessToken, TokenType: "Bearer"}, UserID: string(response.UserID)}, nil
}

type stringID string

func (id *stringID) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*id = stringID(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*id = stringID(number.String())
	return nil
}

func (c *OAuthClient) ExchangeLongLived(ctx context.Context, shortLivedToken string) (socialhub.Token, error) {
	if strings.TrimSpace(shortLivedToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_long_lived", "short-lived token is required")
	}
	body, err := c.do(ctx, http.MethodGet, c.LongTokenURL, url.Values{
		"grant_type": {"th_exchange_token"}, "client_secret": {c.ClientSecret}, "access_token": {shortLivedToken},
	}, false)
	if err != nil {
		return socialhub.Token{}, err
	}
	return c.decodeToken(body, "oauth_long_lived")
}

func (c *OAuthClient) Refresh(ctx context.Context, longLivedToken string) (socialhub.Token, error) {
	if strings.TrimSpace(longLivedToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "long-lived token is required")
	}
	body, err := c.do(ctx, http.MethodGet, c.RefreshURL, url.Values{
		"grant_type": {"th_refresh_token"}, "access_token": {longLivedToken},
	}, false)
	if err != nil {
		return socialhub.Token{}, err
	}
	return c.decodeToken(body, "oauth_refresh")
}

func (c *OAuthClient) do(ctx context.Context, method, endpoint string, values url.Values, form bool) ([]byte, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil || c.Clock == nil {
		return nil, invalidArgument("oauth_token", "OAuth client is incomplete")
	}
	if !validEndpoint(endpoint) {
		return nil, invalidArgument("oauth_token", "token endpoint is invalid")
	}
	var body io.Reader
	if form {
		body = strings.NewReader(values.Encode())
	} else {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return nil, invalidArgument("oauth_token", "token endpoint is invalid")
		}
		query := parsed.Query()
		for key, entries := range values {
			for _, value := range entries {
				query.Add(key, value)
			}
		}
		parsed.RawQuery = query.Encode()
		endpoint = parsed.String()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, platformError("oauth_token", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	if form {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, platformError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return nil, platformError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(responseBody)) > maxOAuthResponseBytes {
		return nil, platformError("oauth_token", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	return responseBody, nil
}

func (c *OAuthClient) decodeToken(body []byte, operation string) (socialhub.Token, error) {
	var response struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil || response.AccessToken == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	tokenType := response.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	var expiresAt time.Time
	if response.ExpiresIn > 0 {
		if response.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
			return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("token expiry is out of range"))
		}
		expiresAt = c.Clock.Now().Add(time.Duration(response.ExpiresIn) * time.Second)
	}
	return socialhub.Token{AccessToken: response.AccessToken, TokenType: tokenType, ExpiresAt: expiresAt}, nil
}
