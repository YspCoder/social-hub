package tumblr

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

// OAuthClient implements Tumblr OAuth2 authorization-code, refresh-token, and
// client-credentials grants.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (c *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if c.ClientID == "" || redirectURI == "" || state == "" || !validOAuthScopes(scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, and valid scopes are required")
	}
	if !validEndpoint(redirectURI) {
		return "", invalidArgument("oauth_authorize", "redirect URI must be an absolute HTTP(S) URL without credentials")
	}
	parsed, err := url.Parse(c.AuthURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", invalidArgument("oauth_authorize", "authorization URL is invalid")
	}
	query := parsed.Query()
	query.Set("client_id", c.ClientID)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	query.Set("redirect_uri", redirectURI)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if strings.TrimSpace(code) == "" || !validEndpoint(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "code and valid redirect URI are required")
	}
	return c.token(ctx, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
	}, "oauth_exchange", "")
}

func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	return c.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}, "oauth_refresh", refreshToken)
}

func (c *OAuthClient) ClientCredentials(ctx context.Context, scopes []string) (socialhub.Token, error) {
	if !validOAuthScopes(scopes) {
		return socialhub.Token{}, invalidArgument("oauth_client_credentials", "valid scopes are required")
	}
	return c.token(ctx, url.Values{
		"grant_type": {"client_credentials"}, "scope": {strings.Join(scopes, " ")},
	}, "oauth_client_credentials", "")
}

func (c *OAuthClient) token(ctx context.Context, values url.Values, operation, retainedRefreshToken string) (socialhub.Token, error) {
	if c.ClientID == "" || c.ClientSecret == "" || c.HTTPClient == nil || c.Clock == nil || !validEndpoint(c.TokenURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	values.Set("client_id", c.ClientID)
	values.Set("client_secret", c.ClientSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		var oauthError struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &oauthError)
		if oauthError.Error != "" {
			return socialhub.Token{}, &socialhub.Error{
				Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction, Platform: "tumblr", Product: productName,
				Op: operation, HTTPStatus: response.StatusCode, PlatformCode: oauthError.Error,
				PlatformMessage: boundedMessage(oauthError.Description, 512),
			}
		}
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.AccessToken) == "" {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.ExpiresIn < 0 || payload.ExpiresIn > int64((365*24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("token expiry is out of range"))
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = c.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	tokenType := payload.TokenType
	if strings.EqualFold(tokenType, "bearer") || tokenType == "" {
		tokenType = "Bearer"
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: firstNonEmpty(payload.RefreshToken, retainedRefreshToken),
		TokenType: tokenType, ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope),
	}, nil
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 {
		return false
	}
	for _, scope := range scopes {
		switch scope {
		case "basic", "write", "offline_access":
		default:
			return false
		}
	}
	return true
}
