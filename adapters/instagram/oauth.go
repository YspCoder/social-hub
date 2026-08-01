package instagram

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

const maxOAuthResponseBytes = 1 << 20

// OAuthClient implements Instagram Login's authorization-code and long-lived
// token flows.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	LongTokenURL string
	RefreshURL   string
	HTTPClient   *http.Client
}

// TokenResult includes the app-scoped Instagram user ID returned by the
// authorization-code exchange.
type TokenResult struct {
	Token       socialhub.Token
	UserID      string
	Permissions []string
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" {
		return "", fmt.Errorf("instagram: client ID, redirect URI, and state are required")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("instagram: invalid authorization URL")
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
		return TokenResult{}, fmt.Errorf("instagram: code and redirect URI are required")
	}
	body, err := c.do(ctx, http.MethodPost, c.TokenURL, url.Values{
		"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "grant_type": {"authorization_code"},
		"redirect_uri": {redirectURI}, "code": {code},
	}, true)
	if err != nil {
		return TokenResult{}, err
	}
	var payload struct {
		AccessToken string   `json:"access_token"`
		UserID      stringID `json:"user_id"`
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" || payload.UserID == "" {
		return TokenResult{}, wrapError("oauth_exchange", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return TokenResult{Token: socialhub.Token{AccessToken: payload.AccessToken, TokenType: "Bearer", Scopes: payload.Permissions}, UserID: string(payload.UserID), Permissions: payload.Permissions}, nil
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
	if shortLivedToken == "" {
		return socialhub.Token{}, fmt.Errorf("instagram: short-lived token is required")
	}
	body, err := c.do(ctx, http.MethodGet, c.LongTokenURL, url.Values{
		"grant_type": {"ig_exchange_token"}, "client_secret": {c.ClientSecret}, "access_token": {shortLivedToken},
	}, false)
	if err != nil {
		return socialhub.Token{}, err
	}
	return decodeOAuthToken(body, "oauth_long_lived")
}

func (c *OAuthClient) Refresh(ctx context.Context, longLivedToken string) (socialhub.Token, error) {
	if longLivedToken == "" {
		return socialhub.Token{}, fmt.Errorf("instagram: long-lived token is required")
	}
	body, err := c.do(ctx, http.MethodGet, c.RefreshURL, url.Values{
		"grant_type": {"ig_refresh_token"}, "access_token": {longLivedToken},
	}, false)
	if err != nil {
		return socialhub.Token{}, err
	}
	return decodeOAuthToken(body, "oauth_refresh")
}

func (c *OAuthClient) do(ctx context.Context, method, endpoint string, values url.Values, form bool) ([]byte, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil {
		return nil, fmt.Errorf("instagram: incomplete OAuth client")
	}
	var body io.Reader
	if form {
		body = strings.NewReader(values.Encode())
	} else {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		parsed.RawQuery = values.Encode()
		endpoint = parsed.String()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if form {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, wrapError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	responseBody, err := readOAuthResponse(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, decodeHTTPError(response.StatusCode, response.Header, responseBody)
	}
	return responseBody, nil
}

func decodeOAuthToken(body []byte, operation string) (socialhub.Token, error) {
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return socialhub.Token{}, wrapError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return socialhub.Token{AccessToken: payload.AccessToken, TokenType: tokenType, ExpiresAt: expiresAt}, nil
}

func readOAuthResponse(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxOAuthResponseBytes+1))
	if err != nil {
		return nil, wrapError("oauth_token", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if len(body) > maxOAuthResponseBytes {
		return nil, wrapError("oauth_token", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return body, nil
}
