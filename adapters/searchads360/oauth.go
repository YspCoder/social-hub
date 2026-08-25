package searchads360

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxOAuthResponseBytes int64 = 1 << 20

// OAuthClient implements Google's OAuth2 web-server flow with offline access.
type OAuthClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if !validOpaque(client.clientID, 1024) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, or authorization endpoint is invalid")
	}
	parsed, _ := url.Parse(defaultAuthURL)
	query := parsed.Query()
	query.Set("client_id", client.clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", reportingScope)
	query.Set("state", state)
	query.Set("access_type", "offline")
	query.Set("include_granted_scopes", "true")
	query.Set("prompt", "consent")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	values := url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}
	return client.token(ctx, "oauth_exchange", values, "", code)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 4096) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	values := url.Values{
		"client_id": {client.clientID}, "client_secret": {client.clientSecret},
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}
	return client.token(ctx, "oauth_refresh", values, refreshToken, refreshToken)
}

func (client *OAuthClient) token(
	ctx context.Context,
	operation string,
	values url.Values,
	existingRefreshToken string,
	redactions ...string,
) (socialhub.Token, error) {
	if !validOpaque(client.clientID, 1024) || !validOpaque(client.clientSecret, 4096) || client.httpClient == nil || client.clock == nil {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.httpClient.Do(request)
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
	var payload struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		TokenType    string `json:"token_type"`
		Error        string `json:"error"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	sensitive := append([]string{client.clientID, client.clientSecret}, redactions...)
	if response.StatusCode < 200 || response.StatusCode >= 300 || payload.Error != "" {
		platformCode := ""
		if decodeErr == nil {
			platformCode = payload.Error
		}
		return socialhub.Token{}, oauthError(operation, response.StatusCode, response.Header, platformCode, client.clock.Now(), sensitive)
	}
	if decodeErr != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if !validOpaque(payload.AccessToken, 4096) || !validOpaque(refreshToken, 4096) ||
		payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer") {
		return socialhub.Token{}, platformContractError(operation, "Google OAuth returned an unsupported token type")
	}
	scopes := strings.Fields(payload.Scope)
	if !validReturnedScopes(scopes) {
		return socialhub.Token{}, platformContractError(operation, "Google OAuth returned invalid scope metadata")
	}
	now := client.clock.Now()
	if now.Unix() <= 0 {
		return socialhub.Token{}, invalidArgument(operation, "clock must return a time after the Unix epoch")
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: now.Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: scopes,
	}, nil
}

func oauthError(operation string, status int, header http.Header, platformCode string, now time.Time, redactions []string) error {
	platformCode = validPlatformCode(platformCode)
	code, class := classifyError(status, "", platformCode)
	switch platformCode {
	case "invalid_client", "invalid_grant", "unauthorized_client", "access_denied":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "invalid_request", "unsupported_grant_type", "invalid_scope":
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "temporarily_unavailable", "server_error":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: "Google OAuth token request failed",
		RequestID:  safeRequestID(header, redactions),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return false
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return true
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
