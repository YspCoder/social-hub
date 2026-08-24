package ads

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

// OAuthClient implements Reddit OAuth 2.0 authorization-code and refresh grants.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserAgent    string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	if !validOpaque(client.ClientID, 1024) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) ||
		!validEndpoint(client.AuthURL) || !validOAuthScopes(scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, scopes, or authorization endpoint is invalid")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("duration", "permanent")
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (client *OAuthClient) Exchange(ctx context.Context, code, redirectURI string) (socialhub.Token, error) {
	if !validOpaque(code, 4096) || !validCallbackURL(redirectURI) {
		return socialhub.Token{}, invalidArgument("oauth_exchange", "authorization code and redirect URI are required")
	}
	return client.token(ctx, "oauth_exchange", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI},
	}, "")
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 8192) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is invalid")
	}
	return client.token(ctx, "oauth_refresh", url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}, refreshToken)
}

func (client *OAuthClient) token(ctx context.Context, operation string, values url.Values, existingRefreshToken string) (socialhub.Token, error) {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 4096) || client.HTTPClient == nil ||
		client.Clock == nil || !validEndpoint(client.TokenURL) || !validUserAgent(client.UserAgent) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.SetBasicAuth(client.ClientID, client.ClientSecret)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", client.UserAgent)
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
		return socialhub.Token{}, decodeOAuthHTTPError(operation, response.StatusCode, response.Header, body, client.Clock.Now())
	}
	var payload struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		TokenType        string `json:"token_type"`
		ExpiresIn        int64  `json:"expires_in"`
		Scope            string `json:"scope"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if payload.Error != "" {
		return socialhub.Token{}, &socialhub.Error{
			Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			PlatformCode: boundedMessage(payload.Error, 256), PlatformMessage: boundedMessage(redactSensitive(payload.ErrorDescription), 512),
		}
	}
	if !validOpaque(payload.AccessToken, 8192) || payload.RefreshToken != "" && !validOpaque(payload.RefreshToken, 8192) || payload.ExpiresIn < 0 {
		return socialhub.Token{}, platformContractError(operation, "Reddit returned an invalid OAuth token payload")
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	if !validOpaque(tokenType, 64) {
		return socialhub.Token{}, platformContractError(operation, "Reddit returned an invalid OAuth token type")
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	var expiresAt time.Time
	if payload.ExpiresIn > 0 {
		expiresAt = client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: tokenType,
		ExpiresAt: expiresAt, Scopes: strings.Fields(payload.Scope),
	}, nil
}

func decodeOAuthHTTPError(operation string, status int, header http.Header, body []byte, now time.Time) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &payload)
	if payload.Error == "" {
		result := decodeHTTPError(status, header, body, now)
		if typed, ok := result.(*socialhub.Error); ok {
			typed.Op = operation
		}
		return result
	}
	code, class := classifyOAuthError(status, payload.Error)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, HTTPStatus: status,
		PlatformCode: boundedMessage(payload.Error, 256), PlatformMessage: boundedMessage(redactSensitive(payload.ErrorDescription), 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-request-id"), header.Get("x-correlation-id")), 256),
		RetryAfter: retryDelay(header, now),
	}
}

func classifyOAuthError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch strings.ToLower(platformCode) {
	case "invalid_client", "invalid_grant", "invalid_token", "unauthorized_client":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	return classifyError(status)
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 4 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		switch scope {
		case readScope, editScope, "adsconversions", "adsdatadeletion":
		default:
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}
