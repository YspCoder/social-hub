package marketing

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

// OAuthClient implements Snapchat OAuth 2.0 authorization-code and
// refresh-token grants for the Marketing API.
type OAuthClient struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	HTTPClient   *http.Client
	Clock        socialhub.Clock
}

func (client *OAuthClient) AuthorizationURL(redirectURI, state string) (string, error) {
	if !validOpaque(client.ClientID, 1024) || !validCallbackURL(redirectURI) || !validOpaque(state, 1024) || !validEndpoint(client.AuthURL) {
		return "", invalidArgument("oauth_authorize", "client ID, redirect URI, state, or authorization endpoint is invalid")
	}
	parsed, _ := url.Parse(client.AuthURL)
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", client.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", marketingScope)
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
	if !validOpaque(client.ClientID, 1024) || !validOpaque(client.ClientSecret, 4096) || client.HTTPClient == nil || client.Clock == nil || !validEndpoint(client.TokenURL) {
		return socialhub.Token{}, invalidArgument(operation, "OAuth client is incomplete")
	}
	values.Set("client_id", client.ClientID)
	values.Set("client_secret", client.ClientSecret)
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
		code, class := classifyError(http.StatusUnauthorized, payload.Error, payload.ErrorDescription)
		return socialhub.Token{}, &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
			PlatformCode: boundedMessage(payload.Error, 256), PlatformMessage: boundedMessage(redactSensitive(payload.ErrorDescription), 512),
		}
	}
	if !validOpaque(payload.AccessToken, 8192) || payload.ExpiresIn <= 0 || payload.ExpiresIn > int64((24*time.Hour)/time.Second) {
		return socialhub.Token{}, platformContractError(operation, "Snapchat returned an invalid OAuth token response")
	}
	refreshToken := payload.RefreshToken
	if refreshToken == "" {
		refreshToken = existingRefreshToken
	}
	if refreshToken != "" && !validOpaque(refreshToken, 8192) {
		return socialhub.Token{}, platformContractError(operation, "Snapchat returned an invalid refresh token")
	}
	tokenType := payload.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	scopes := strings.Fields(payload.Scope)
	return socialhub.Token{
		AccessToken: payload.AccessToken, RefreshToken: refreshToken, TokenType: tokenType,
		ExpiresAt: client.Clock.Now().Add(time.Duration(payload.ExpiresIn) * time.Second), Scopes: scopes,
	}, nil
}
