package strava

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

var oauthScopes = map[string]struct{}{
	"read": {}, "read_all": {}, "profile:read_all": {}, "profile:write": {},
	"activity:read": {}, "activity:read_all": {}, "activity:write": {},
}

// OAuthClient implements Strava's authorization-code flow and rotating
// refresh-token grant.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validResourceID(client.ClientID) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) || len(scopes) == 0 || !validEndpoint(client.AuthURL) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, scopes, or authorization endpoint is invalid")
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, found := oauthScopes[scope]; !found {
			return "", invalidArgument("oauth_authorize", "scope is not documented by Strava API v3")
		}
		if _, duplicate := seen[scope]; duplicate {
			return "", invalidArgument("oauth_authorize", "scopes must not contain duplicates")
		}
		seen[scope] = struct{}{}
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("scope", strings.Join(scopes, ","))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code is required")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
		"code": {code}, "grant_type": {"authorization_code"},
	})
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 4096) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	return client.token(ctx, "oauth_refresh", url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
		"refresh_token": {refreshToken}, "grant_type": {"refresh_token"},
	})
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values) (socialhub.Token, error) {
	if !validResourceID(client.ClientID) || !validOpaque(client.ClientSecret, 4096) || client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOAuthResponseBytes+1))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxOAuthResponseBytes {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("OAuth response exceeded size limit"))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return socialhub.Token{}, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresAt    int64  `json:"expires_at"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if !validOpaque(payload.AccessToken, 4096) || !validOpaque(payload.RefreshToken, 4096) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := client.Clock.Now()
	expiresAt := time.Unix(payload.ExpiresAt, 0).UTC()
	if payload.ExpiresAt <= 0 && payload.ExpiresIn > 0 {
		expiresAt = now.Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	if !expiresAt.After(now) || expiresAt.After(now.Add(7*24*time.Hour)) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	if !validOpaque(tokenType, 64) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: tokenType,
		ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope),
	}, nil
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}
