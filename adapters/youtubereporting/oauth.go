package youtubereporting

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

// OAuthClient implements Google's user OAuth2 web-server authorization-code
// flow with offline access and refresh tokens.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
	Scopes       []string
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if !validOpaque(client.ClientID, 1024) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) ||
		!validEndpoint(client.AuthURL) || !validOAuthScopes(client.Scopes) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, authorization endpoint, or YouTube Reporting scopes are invalid")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(client.Scopes, " "))
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
	if err := client.valid("oauth_exchange"); err != nil {
		return socialhub.Token{}, err
	}
	values := url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret}, "code": {code},
		"grant_type": {"authorization_code"}, "redirect_uri": {redirectURI},
	}
	return requestToken(ctx, client.HTTPClient, client.TokenURL, client.Clock, "oauth_exchange", values, "", true, client.Scopes)
}

func (client *OAuthClient) Refresh(ctx context.Context, refreshToken string) (socialhub.Token, error) {
	if !validOpaque(refreshToken, 16384) {
		return socialhub.Token{}, invalidArgument("oauth_refresh", "refresh token is required")
	}
	if err := client.valid("oauth_refresh"); err != nil {
		return socialhub.Token{}, err
	}
	values := url.Values{
		"client_id": {client.ClientID}, "client_secret": {client.ClientSecret},
		"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
	}
	return requestToken(ctx, client.HTTPClient, client.TokenURL, client.Clock, "oauth_refresh", values, refreshToken, false, client.Scopes)
}

func (client *OAuthClient) valid(operation string) error {
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 16384) || client.HTTPClient == nil ||
		client.Clock == nil || !validEndpoint(client.TokenURL) || !validOAuthScopes(client.Scopes) {
		return invalidArgument(operation, "OAuth client is incomplete")
	}
	return nil
}

func requestToken(
	ctx context.Context,
	httpClient *http.Client,
	tokenURL string,
	clock socialhub.Clock,
	operation string,
	values url.Values,
	existingRefreshToken string,
	requireRefresh bool,
	scopes []string,
) (socialhub.Token, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := httpClient.Do(request)
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
		AccessToken      string `json:"access_token"`
		ExpiresIn        int64  `json:"expires_in"`
		RefreshToken     string `json:"refresh_token"`
		Scope            string `json:"scope"`
		TokenType        string `json:"token_type"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	decodeErr := json.Unmarshal(body, &payload)
	if response.StatusCode < 200 || response.StatusCode >= 300 || payload.Error != "" {
		if decodeErr != nil {
			return socialhub.Token{}, withOperation(decodeHTTPError(response.StatusCode, response.Header, body), operation)
		}
		return socialhub.Token{}, oauthError(operation, response.StatusCode, response.Header, payload.Error, payload.ErrorDescription)
	}
	if decodeErr != nil {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, decodeErr)
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if !validOpaque(payload.AccessToken, 16384) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) ||
		(payload.TokenType != "" && !strings.EqualFold(payload.TokenType, "bearer")) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid token response fields"))
	}
	if requireRefresh && !validOpaque(refreshToken, 16384) {
		return socialhub.Token{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("offline OAuth response omitted refresh token"))
	}
	grantedScopes := strings.Fields(payload.Scope)
	if len(grantedScopes) == 0 {
		grantedScopes = append([]string(nil), scopes...)
	}
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: "Bearer",
		ExpiresAt: clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: grantedScopes,
	}, nil
}

func oauthError(operation string, status int, header http.Header, platformCode, message string) error {
	code, class := classifyError(status, "", "")
	switch strings.ToLower(strings.TrimSpace(platformCode)) {
	case "invalid_client", "invalid_grant", "unauthorized_client":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "access_denied", "insufficient_scope":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "temporarily_unavailable", "server_error":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(redactSensitive(platformCode), 256),
		PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("x-goog-request-id"), header.Get("x-google-request-id"), header.Get("x-request-id")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}
